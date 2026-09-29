package probe

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/amnezia-vpn/amneziawg-go/v3/conn"
	"github.com/amnezia-vpn/amneziawg-go/v3/device"
	"github.com/amnezia-vpn/amneziawg-go/v3/tun/netstack"
)

const awgTestConfig = `[Interface]
PrivateKey = AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=
Address = 10.8.1.2/32
DNS = 1.1.1.1, 1.0.0.1
MTU = 1280
Jc = 3
Jmin = 67
Jmax = 170
S1 = 79
S2 = 137
S3 = 26
S4 = 5
H1 = 1001-2000
H2 = 2001-3000
H3 = 3001-4000
H4 = 4001-5000
I1 = <r 2><b 0x01020304><rc 38>
[Peer]
PublicKey = AQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQE=
Endpoint = 127.0.0.1:9
AllowedIPs = 0.0.0.0/0, ::/0
PersistentKeepalive = 33
`

func TestAWGParametersPreserved(t *testing.T) {
	c, err := parseAWGConfig([]byte(awgTestConfig))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"s3=26\n", "s4=5\n", "h1=1001-2000\n", "h4=4001-5000\n", "i1=<r 2><b 0x01020304><rc 38>\n", "persistent_keepalive_interval=33\n", "allowed_ip=::/0\n"} {
		if !strings.Contains(c.ipc, want) {
			t.Errorf("missing parameter %q", want)
		}
	}
	if len(c.addresses) != 1 || len(c.dns) != 2 || c.mtu != 1280 {
		t.Fatal("network configuration lost")
	}
}

func TestAWGHTTPSessionThroughRealTunnel(t *testing.T) {
	serverKey, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	clientKey, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tun, network, err := netstack.CreateNetTUN([]netip.Addr{netip.MustParseAddr("10.8.1.1")}, nil, 1280)
	if err != nil {
		t.Fatal(err)
	}
	dev := device.NewDevice(tun, conn.NewDefaultBind(), device.NewLogger(device.LogLevelSilent, ""))
	defer dev.Close()
	// Same AWG 2 padding and header ranges as the fixture, with ephemeral keys.
	ipc := fmt.Sprintf("private_key=%s\nlisten_port=0\ns1=79\ns2=137\ns3=26\ns4=5\nh1=1001-2000\nh2=2001-3000\nh3=3001-4000\nh4=4001-5000\npublic_key=%s\nallowed_ip=10.8.1.2/32\n", hex.EncodeToString(serverKey.Bytes()), hex.EncodeToString(clientKey.PublicKey().Bytes()))
	if err := configureAWGDevice(dev, ipc); err != nil {
		t.Fatal(err)
	}
	if err := dev.Up(); err != nil {
		t.Fatal(err)
	}
	state, err := dev.IpcGet()
	if err != nil {
		t.Fatal(err)
	}
	var port string
	for _, line := range strings.Split(state, "\n") {
		if value, ok := strings.CutPrefix(line, "listen_port="); ok {
			port = value
		}
	}
	if port == "" || port == "0" {
		t.Fatal("fixture server has no UDP port")
	}
	listener, err := network.ListenTCPAddrPort(netip.MustParseAddrPort("10.8.1.1:80"))
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "tunnel-ok") })}
	go server.Serve(listener)
	defer server.Close()
	raw := strings.ReplaceAll(awgTestConfig, "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=", base64.StdEncoding.EncodeToString(clientKey.Bytes()))
	raw = strings.ReplaceAll(raw, "AQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQE=", base64.StdEncoding.EncodeToString(serverKey.PublicKey().Bytes()))
	raw = strings.ReplaceAll(raw, "127.0.0.1:9", "127.0.0.1:"+port)
	var stats awgProbeStats
	dial, cleanup, err := newAWGDialer(context.Background(), "awg://"+base64.StdEncoding.EncodeToString([]byte(raw)), &stats)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	transport := &http.Transport{DialContext: dial}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	response, err := client.Get("http://10.8.1.1/")
	if err != nil {
		t.Logf("client stats: %v", stats.snapshot())
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil || string(body) != "tunnel-ok" {
		t.Fatal("no valid response through AWG tunnel")
	}
	statsResult := stats.snapshot()
	if statsResult["last_handshake_time_sec"] <= 0 || statsResult["rx_bytes"] <= 0 {
		t.Fatal("missing handshake or received-byte metric")
	}
}

func TestAWGRejectInvalidSecretsAndPaths(t *testing.T) {
	bad := strings.Replace(awgTestConfig, "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=", "DO_NOT_DISCLOSE", 1)
	_, err := parseAWGConfig([]byte(bad))
	if err == nil || strings.Contains(err.Error(), "DO_NOT_DISCLOSE") {
		t.Fatal("invalid key accepted or disclosed")
	}
	for _, name := range []string{"../example", "/etc/passwd", "example.conf", "example?x=y"} {
		_, _, err := newAWGDialer(context.Background(), "awgfile://"+name)
		if err == nil || err.Error() != "invalid AWG profile name" {
			t.Errorf("invalid path accepted: %s", name)
		}
	}
}

func TestAWGEngineAndCancellation(t *testing.T) {
	uri := "awg://" + base64.StdEncoding.EncodeToString([]byte(awgTestConfig))
	var stats awgProbeStats
	dial, closeTunnel, err := newAWGDialer(context.Background(), uri, &stats)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	// With no server on the endpoint, DNS/connect must time out through the tunnel.
	if c, err := dial(ctx, "tcp", "1.1.1.1:443"); err == nil {
		c.Close()
		t.Fatal("unexpected direct connectivity")
	}
	_, _, err = newAWGDialer(ctx, uri)
	if err == nil {
		t.Fatal("blocked probe ignored cancellation")
	}
	snapshot := stats.snapshot()
	if len(snapshot) != 3 || snapshot["last_handshake_time_sec"] != 0 {
		t.Fatal("unexpected stats fields or false handshake success")
	}
	closeTunnel()
	// Verify that cleanup releases both tunnel resources and the serialization slot.
	_, closeAgain, err := newAWGDialer(context.Background(), uri)
	if err != nil {
		t.Fatal(err)
	}
	closeAgain()
}

func TestAWGPeerLocksIndependentAndReclaimed(t *testing.T) {
	first, err := acquireAWGPeer(context.Background(), [32]byte{1})
	if err != nil {
		t.Fatal(err)
	}
	second, err := acquireAWGPeer(context.Background(), [32]byte{2})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if release, err := acquireAWGPeer(ctx, [32]byte{1}); err == nil {
		release()
		t.Fatal("same peer did not wait")
	}
	first()
	second()
	awgProbeLocks.Lock()
	defer awgProbeLocks.Unlock()
	if len(awgProbeLocks.peers) != 0 {
		t.Fatal("peer lock entries leaked")
	}
}
