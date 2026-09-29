package probe

// Native AmneziaWG probes preserve AWG 2 parameter ranges and signature chains.
// A fresh userspace tunnel is owned by each probe; host routes are never changed.
import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/amnezia-vpn/amneziawg-go/v3/conn"
	"github.com/amnezia-vpn/amneziawg-go/v3/device"
	"github.com/amnezia-vpn/amneziawg-go/v3/tun/netstack"
	"gopkg.in/ini.v1"
)

var awgProbeLocks = struct {
	sync.Mutex
	peers map[[32]byte]*awgPeerLock
}{peers: make(map[[32]byte]*awgPeerLock)}

type awgPeerLock struct {
	slot chan struct{}
	refs int
}

// Retain entries only while probes are active or waiting. A hash avoids keeping
// private keys in the map, and unrelated clients do not block one another.
func acquireAWGPeer(ctx context.Context, key [32]byte) (func(), error) {
	awgProbeLocks.Lock()
	entry := awgProbeLocks.peers[key]
	if entry == nil {
		entry = &awgPeerLock{slot: make(chan struct{}, 1)}
		awgProbeLocks.peers[key] = entry
	}
	entry.refs++
	awgProbeLocks.Unlock()
	drop := func() {
		awgProbeLocks.Lock()
		defer awgProbeLocks.Unlock()
		entry.refs--
		if entry.refs == 0 {
			delete(awgProbeLocks.peers, key)
		}
	}
	select {
	case entry.slot <- struct{}{}:
		return func() { <-entry.slot; drop() }, nil
	case <-ctx.Done():
		drop()
		return nil, ctx.Err()
	}
}

var awgProfileName = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

type awgSettings struct {
	addresses, dns []netip.Addr
	mtu            int
	ipc            string
	identity       [32]byte
	endpointHost   string
	endpointPort   string
}

type awgProbeStats struct {
	device *device.Device
}

// UAPI also contains keys, so expose only an explicit allowlist of counters.
func (s *awgProbeStats) snapshot() map[string]float64 {
	out := map[string]float64{"last_handshake_time_sec": 0, "rx_bytes": 0, "tx_bytes": 0}
	if s.device == nil {
		return out
	}
	raw, err := s.device.IpcGet()
	if err != nil {
		return out
	}
	for _, line := range strings.Split(raw, "\n") {
		key, value, ok := strings.Cut(line, "=")
		if _, allowed := out[key]; ok && allowed {
			if number, err := strconv.ParseFloat(value, 64); err == nil {
				out[key] = number
			}
		}
	}
	return out
}

// netstack emits EventUp at creation. Apply device obfuscation before adding a
// peer: its keepalive may immediately send a handshake if EventUp already ran.
func configureAWGDevice(dev *device.Device, ipc string) error {
	deviceConfig, peerConfig, ok := strings.Cut(ipc, "public_key=")
	if !ok {
		return errors.New("AWG profile has no peer")
	}
	if err := dev.IpcSet(deviceConfig); err != nil {
		return err
	}
	return dev.IpcSet("public_key=" + peerConfig)
}

func awgKey(value string) (string, error) {
	b, err := base64.StdEncoding.DecodeString(value)
	if err != nil || len(b) != 32 {
		return "", errors.New("invalid AWG key")
	}
	return hex.EncodeToString(b), nil
}

func parseAWGConfig(raw []byte) (awgSettings, error) {
	out := awgSettings{mtu: 1280}
	cfg, err := ini.LoadSources(ini.LoadOptions{IgnoreInlineComment: true}, raw)
	if err != nil {
		return out, errors.New("invalid AWG INI configuration")
	}
	iface, peer := cfg.Section("Interface"), cfg.Section("Peer")
	for _, value := range strings.Split(iface.Key("Address").String(), ",") {
		prefix, err := netip.ParsePrefix(strings.TrimSpace(value))
		if err != nil {
			return out, errors.New("invalid AWG Address")
		}
		out.addresses = append(out.addresses, prefix.Addr())
	}
	for _, value := range strings.Split(iface.Key("DNS").String(), ",") {
		if strings.TrimSpace(value) == "" {
			continue
		}
		ip, err := netip.ParseAddr(strings.TrimSpace(value))
		if err != nil {
			return out, errors.New("AWG DNS must contain IP addresses")
		}
		out.dns = append(out.dns, ip)
	}
	if iface.HasKey("MTU") {
		out.mtu, err = iface.Key("MTU").Int()
		if err != nil || out.mtu < 576 || out.mtu > 9000 {
			return out, errors.New("invalid AWG MTU")
		}
	}
	private, err := awgKey(iface.Key("PrivateKey").String())
	if err != nil {
		return out, err
	}
	public, err := awgKey(peer.Key("PublicKey").String())
	if err != nil {
		return out, err
	}
	out.identity = sha256.Sum256([]byte(private + public))
	var b strings.Builder
	fmt.Fprintf(&b, "private_key=%s\n", private)
	for _, key := range []string{"Jc", "Jmin", "Jmax", "S1", "S2", "S3", "S4", "H1", "H2", "H3", "H4", "I1", "I2", "I3", "I4", "I5"} {
		if iface.HasKey(key) {
			value := iface.Key(key).String()
			if strings.ContainsAny(value, "\r\n") {
				return out, errors.New("invalid multiline AWG parameter")
			}
			fmt.Fprintf(&b, "%s=%s\n", strings.ToLower(key), value)
		}
	}
	fmt.Fprintf(&b, "replace_peers=true\npublic_key=%s\n", public)
	if peer.HasKey("PresharedKey") {
		psk, err := awgKey(peer.Key("PresharedKey").String())
		if err != nil {
			return out, err
		}
		fmt.Fprintf(&b, "preshared_key=%s\n", psk)
	}
	host, port, err := net.SplitHostPort(peer.Key("Endpoint").String())
	if err != nil || host == "" || strings.ContainsAny(host, "\r\n") {
		return out, errors.New("invalid AWG Endpoint")
	}
	portNumber, err := strconv.ParseUint(port, 10, 16)
	if err != nil || portNumber == 0 {
		return out, errors.New("invalid AWG Endpoint port")
	}
	out.endpointHost, out.endpointPort = host, port
	b.WriteString("replace_allowed_ips=true\n")
	for _, value := range strings.Split(peer.Key("AllowedIPs").String(), ",") {
		prefix, err := netip.ParsePrefix(strings.TrimSpace(value))
		if err != nil {
			return out, errors.New("invalid AWG AllowedIPs")
		}
		fmt.Fprintf(&b, "allowed_ip=%s\n", prefix)
	}
	if peer.HasKey("PersistentKeepalive") {
		seconds, err := strconv.ParseUint(peer.Key("PersistentKeepalive").String(), 10, 16)
		if err != nil {
			return out, errors.New("invalid AWG PersistentKeepalive")
		}
		fmt.Fprintf(&b, "persistent_keepalive_interval=%d\n", seconds)
	}
	out.ipc = b.String()
	return out, nil
}

func newAWGDialer(ctx context.Context, connection string, stats ...*awgProbeStats) (func(context.Context, string, string) (net.Conn, error), func(), error) {
	var raw []byte
	var err error
	if name, ok := strings.CutPrefix(connection, "awgfile://"); ok {
		if !awgProfileName.MatchString(name) {
			return nil, nil, errors.New("invalid AWG profile name")
		}
		dir := os.Getenv("WHITEBOX_AWG_CONFIG_DIR")
		if dir == "" {
			dir = "/etc/whitebox/clients"
		}
		raw, err = os.ReadFile(filepath.Join(dir, name+".conf"))
	} else {
		raw, err = base64.StdEncoding.DecodeString(strings.TrimPrefix(connection, "awg://"))
	}
	if err != nil {
		return nil, nil, errors.New("cannot read AWG profile")
	}
	settings, err := parseAWGConfig(raw)
	if err != nil {
		return nil, nil, err
	}
	// Resolve only the VPN endpoint with the host resolver. Target DNS goes
	// through the netstack and its configured in-tunnel DNS servers.
	endpointIP, err := netip.ParseAddr(settings.endpointHost)
	if err != nil {
		addresses, resolveErr := net.DefaultResolver.LookupNetIP(ctx, "ip", settings.endpointHost)
		if resolveErr != nil || len(addresses) == 0 {
			return nil, nil, errors.New("cannot resolve AWG Endpoint")
		}
		endpointIP = addresses[0]
	}
	settings.ipc = strings.Replace(settings.ipc, "replace_allowed_ips=true\n",
		"endpoint="+net.JoinHostPort(endpointIP.String(), settings.endpointPort)+"\nreplace_allowed_ips=true\n", 1)
	// Serialize probes so the same peer cannot roam between simultaneous tunnels.
	release, err := acquireAWGPeer(ctx, settings.identity)
	if err != nil {
		return nil, nil, err
	}
	tun, network, err := netstack.CreateNetTUN(settings.addresses, settings.dns, settings.mtu)
	if err != nil {
		release()
		return nil, nil, errors.New("cannot create AWG netstack")
	}
	dev := device.NewDevice(tun, conn.NewDefaultBind(), device.NewLogger(device.LogLevelSilent, ""))
	closeTunnel := func() { dev.Close(); release() }
	if err := configureAWGDevice(dev, settings.ipc); err != nil {
		closeTunnel()
		return nil, nil, errors.New("AWG engine rejected profile parameters")
	}
	if err := dev.Up(); err != nil {
		closeTunnel()
		return nil, nil, errors.New("cannot start AWG tunnel")
	}
	if len(stats) > 0 && stats[0] != nil {
		stats[0].device = dev
	}
	return network.DialContext, closeTunnel, nil
}
