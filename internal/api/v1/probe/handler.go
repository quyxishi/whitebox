package probe

import (
	"bytes"
	"cmp"
	"context"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/quyxishi/whitebox/internal/serial"

	"github.com/gin-gonic/gin"
	"github.com/gvcgo/vpnparser/pkgs/utils"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	_ "github.com/xtls/xray-core/app/dispatcher"
	_ "github.com/xtls/xray-core/app/dns"
	_ "github.com/xtls/xray-core/app/proxyman/inbound"
	_ "github.com/xtls/xray-core/app/proxyman/outbound"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/core"
	_ "github.com/xtls/xray-core/main/json"
	_ "github.com/xtls/xray-core/proxy/vless/outbound"
	_ "github.com/xtls/xray-core/transport/internet/reality"
	_ "github.com/xtls/xray-core/transport/internet/tcp"
)

type ProbeHandler struct {
}

func NewProbeHandler() *ProbeHandler {
	return &ProbeHandler{}
}

type ProbeParams struct {
	Connection   string
	Schema       string
	Target       string
	MaxRedirects int
	TimeoutMs    int
}

func (h *ProbeHandler) parseProbeParams(ctx *gin.Context) (out ProbeParams, ok bool) {
	out.Connection, ok = ctx.GetQuery("ctx")
	if !ok {
		ctx.String(http.StatusBadRequest, "VPN connection query-param 'ctx' is missing")
		return
	}

	out.Schema = cmp.Or(utils.ParseScheme(out.Connection), "<empty>")

	target, ok := ctx.GetQuery("target")
	if !ok {
		ctx.String(http.StatusBadRequest, "Target query-param is missing")
		return
	}
	if !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") {
		target = "http://" + target
	}

	out.Target = target

	out.MaxRedirects = 5
	if v, err := strconv.Atoi(ctx.Query("max_redirects")); err == nil {
		out.MaxRedirects = v
	}

	out.TimeoutMs = 3000
	if v, err := strconv.Atoi(ctx.Query("timeout_ms")); err == nil {
		out.TimeoutMs = v
	}

	return out, true
}

func (h *ProbeHandler) parseXrayConf(ctx *gin.Context, params *ProbeParams) (out *core.Config, ok bool) {
	var config string
	var err error
	switch params.Schema {
	case "http://", "https://":
		log.Println("[DEBUG] probe/handler: assuming that ctx is json subscription link")
		config, err = serial.ParseSubscriptionURI(params.Connection, &serial.ParseSubParams{EnableDebug: true})
	default:
		log.Println("[DEBUG] probe/handler: assuming that ctx is direct vpn connection uri")
		config, err = serial.ParseURI(serial.CONFIG_BACKEND_XRAYCORE, params.Connection, &serial.ParseParams{EnableDebug: true})
	}

	if err != nil {
		ctx.String(http.StatusBadRequest, "Unable to parse uri-based config for xray-core due: %v", err)
		return out, false
	}

	out, err = core.LoadConfig("json", bytes.NewReader([]byte(config)))
	if err != nil {
		ctx.String(http.StatusInternalServerError, "Unable to load xray config: %s", err.Error())
		return out, false
	}

	return out, true
}

func (h *ProbeHandler) Probe(ctx *gin.Context) {
	params, ok := h.parseProbeParams(ctx)
	if !ok {
		return
	}

	// *

	probeSuccessGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tun_probe_success",
		Help: "Displays whether or not the probe over tunnel was a success",
	})

	probeDurationGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tun_probe_duration_seconds",
		Help: "Returns how long the probe took to complete in seconds",
	})

	probeHttpStatusCodeGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tun_probe_http_status_code",
		Help: "Response HTTP status code",
	})

	registry := prometheus.NewRegistry()
	registry.MustRegister(probeSuccessGauge)
	registry.MustRegister(probeDurationGauge)

	// *

	xrayConf, ok := h.parseXrayConf(ctx, &params)
	if !ok {
		return
	}

	// *

	probe_entry := time.Now()

	instance, err := core.New(xrayConf)
	if err != nil {
		ctx.String(http.StatusInternalServerError, "Unable to init xray instance: %s", err.Error())
		return
	}
	if err := instance.Start(); err != nil {
		ctx.String(http.StatusInternalServerError, "Unable to start xray instance: %s", err.Error())
		return
	}

	defer func() {
		if err := instance.Close(); err != nil {
			log.Printf("[ERROR] probe/handler: failed to close xray instance: %v", err)
		}
	}()

	client := &http.Client{
		Timeout: time.Duration(params.TimeoutMs) * time.Millisecond,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				dest, err := net.ParseDestination(network + ":" + addr)
				if err != nil {
					return nil, err
				}

				return core.Dial(ctx, instance, dest)
			},
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > params.MaxRedirects {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}

	success := 1
	resp, err := client.Get(params.Target)
	if err != nil {
		success = 0
		log.Printf("[ERROR] probe/handler: connection failed: %v\n", err)
	}
	if resp != nil {
		defer func() {
			if err := resp.Body.Close(); err != nil {
				log.Printf("[ERROR] probe/handler: failed to close response.body instance: %v", err)
			}
		}()
	}

	probe_elapsed := time.Since(probe_entry).Seconds()
	probeDurationGauge.Set(probe_elapsed)
	probeSuccessGauge.Set(float64(success))

	if resp != nil {
		registry.MustRegister(probeHttpStatusCodeGauge)
		probeHttpStatusCodeGauge.Set(float64(resp.StatusCode))
	}

	// *

	promHandler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	promHandler.ServeHTTP(ctx.Writer, ctx.Request)
}
