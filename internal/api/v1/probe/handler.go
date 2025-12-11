package probe

import (
	"bytes"
	"cmp"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptrace"
	"strconv"
	"strings"
	"time"

	"github.com/quyxishi/whitebox/internal/serial"
	"golang.org/x/net/publicsuffix"

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

type ProbeHandler struct{}

func NewProbeHandler() *ProbeHandler {
	return &ProbeHandler{}
}

type ProbeParams struct {
	Connection   string
	Scheme       string
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

	out.Scheme = cmp.Or(utils.ParseScheme(out.Connection), "<empty>")

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

	out.TimeoutMs = 5000
	if v, err := strconv.Atoi(ctx.Query("timeout_ms")); err == nil {
		out.TimeoutMs = v
	}

	return out, true
}

func (h *ProbeHandler) parseXrayConf(ctx *gin.Context, params *ProbeParams) (out *core.Config, ok bool) {
	var config string
	var err error
	switch params.Scheme {
	case "http://", "https://":
		slog.Debug("assuming that ctx is json subscription link")
		config, err = serial.ParseSubscriptionURI(params.Connection, &serial.ParseSubParams{EnableDebug: true})
	default:
		slog.Debug("assuming that ctx is direct vpn connection uri")
		config, err = serial.ParseURI(serial.CONFIG_BACKEND_XRAYCORE, params.Connection, &serial.ParseParams{EnableDebug: true})
	}

	if err != nil {
		ctx.String(http.StatusBadRequest, "Unable to parse uri-based config for xray-core due: %v", err)
		return out, false
	}

	out, err = core.LoadConfig("json", bytes.NewReader([]byte(config)))
	if err != nil {
		ctx.String(http.StatusInternalServerError, "Unable to load xray config: %v", err)
		return out, false
	}

	return out, true
}

func (h *ProbeHandler) Probe(ctx *gin.Context) {
	// todo!
	//  - duration phase=tunnel metrics
	//  - sublinks support [raw, json]
	//  - metrics for sublinks
	//  - H3/QUIC support

	params, ok := h.parseProbeParams(ctx)
	if !ok {
		return
	}

	xrayConf, ok := h.parseXrayConf(ctx, &params)
	if !ok {
		return
	}

	slog.Info(
		"recv probe w/",
		"scheme", params.Scheme,
		"target", params.Target,
		"method", "GET",
		"maxRedirects", params.MaxRedirects,
		"timeoutMs", params.TimeoutMs,
	)

	// *

	probeSuccessGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tun_probe_success",
		Help: "Displays whether or not the probe over tunnel was a success",
	})

	probeDurationGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tun_probe_duration_seconds",
		Help: "Returns how long the probe took to complete in seconds",
	})

	probeDurationGaugeVec := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "tun_probe_http_duration_seconds",
		Help: "Duration of HTTP request by phase, summed over all traces",
	}, []string{"phase"})

	probeContentLengthGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tun_probe_http_content_length_bytes",
		Help: "Length of HTTP content response in bytes",
	})

	probeBodyUncompressedLengthGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tun_probe_http_uncompressed_body_length_bytes",
		Help: "Length of uncompressed response body in bytes",
	})

	probeRedirectsGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tun_probe_http_redirects",
		Help: "The number of redirects",
	})

	probeIsSslGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tun_probe_http_ssl",
		Help: "Indicates if SSL was used for the final trace",
	})

	probeHttpStatusCodeGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tun_probe_http_status_code",
		Help: "Response HTTP status code",
	})

	for _, lv := range []string{"connect", "tls", "processing", "transfer"} {
		probeDurationGaugeVec.WithLabelValues(lv)
	}

	registry := prometheus.NewRegistry()
	registry.MustRegister(probeSuccessGauge)
	registry.MustRegister(probeDurationGauge)
	registry.MustRegister(probeDurationGaugeVec)
	registry.MustRegister(probeContentLengthGauge)
	registry.MustRegister(probeBodyUncompressedLengthGauge)
	registry.MustRegister(probeRedirectsGauge)
	registry.MustRegister(probeIsSslGauge)

	// *

	instance, err := core.New(xrayConf)
	if err != nil {
		slog.Error("failed to init xray instance", "err", err)
		ctx.String(http.StatusInternalServerError, "Unable to init xray instance: %s", err.Error())
		return
	}
	if err := instance.Start(); err != nil {
		slog.Error("failed to start xray instance", "err", err)
		ctx.String(http.StatusInternalServerError, "Unable to start xray instance: %s", err.Error())
		return
	}
	defer func() {
		if err := instance.Close(); err != nil {
			slog.Error("failed to close xray instance", "err", err)
		}
	}()

	// *

	redirectCounter := RedirectCounter{Max: params.MaxRedirects}

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
		CheckRedirect: redirectCounter.CheckRedirect,
	}

	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		slog.Error("failed to make cookiejar.Jar", "err", err)
		ctx.String(http.StatusInternalServerError, "Unable to make cookiejar.Jar due: %v", err)
		return
	}
	client.Jar = jar

	// inject custom transport that tracks traces for each redirect
	tt := RoundTransport{Transport: client.Transport}
	client.Transport = &tt

	trace := &httptrace.ClientTrace{
		DNSStart:             tt.DNSStart,
		DNSDone:              tt.DNSDone,
		TLSHandshakeStart:    tt.TLSHandshakeStart,
		TLSHandshakeDone:     tt.TLSHandshakeDone,
		ConnectStart:         tt.ConnectStart,
		ConnectDone:          tt.ConnectDone,
		GotConn:              tt.GotConn,
		GotFirstResponseByte: tt.GotFirstResponseByte,
	}

	req, err := http.NewRequest("GET", params.Target, bytes.NewBuffer([]byte{}))
	if err != nil {
		slog.Error("failed to to construct http.Request", "err", err)
		ctx.String(http.StatusInternalServerError, "Unable to construct http.Request due: %v", err)
		return
	}

	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))

	// *

	var probeSuccess float64 = 1
	var probeElapsed float64
	var probeBodyBytes float64

	resp, err := client.Do(req)
	if err != nil {
		probeSuccess = 0
		slog.Error("probe failed", "err", err)
	}
	if resp != nil {
		defer func() {
			if err := resp.Body.Close(); err != nil {
				slog.Error("failed to close response.Body instance", "err", err)
			}
		}()

		byteCounter := &ByteCounter{ReadCloser: resp.Body}

		_, err = io.Copy(io.Discard, byteCounter)
		if err != nil {
			slog.Error("failed to read http response body", "err", err)
			probeSuccess = 0
		}

		tt.Actual.Exit = time.Now()
		probeElapsed = tt.Actual.Exit.Sub(tt.Traces[0].Init).Seconds()
		probeBodyBytes = float64(byteCounter.n)

		if err := byteCounter.Close(); err != nil {
			slog.Error("failed to close byteCounter instance", "err", err)
		}

		registry.MustRegister(probeHttpStatusCodeGauge)
		probeHttpStatusCodeGauge.Set(float64(resp.StatusCode))
		probeContentLengthGauge.Set(float64(resp.ContentLength))

		if resp.TLS != nil {
			probeIsSslGauge.Set(float64(1))
		}

		slog.Info(
			"probe succeeded",
			"scheme", params.Scheme,
			"timeoutMs", params.TimeoutMs,
			"target", resp.Request.URL,
			"method", resp.Request.Method,
			"redirects", redirectCounter.Total,
			"status", resp.Status,
			"contentLength", resp.ContentLength,
			"bodyLength", byteCounter.n,
		)
	}

	// *

	probeDurationGauge.Set(probeElapsed)
	probeSuccessGauge.Set(probeSuccess)
	probeBodyUncompressedLengthGauge.Set(probeBodyBytes)
	probeRedirectsGauge.Set(float64(redirectCounter.Total))

	tt.mu.Lock()
	defer tt.mu.Unlock()

	slog.Info(fmt.Sprintf("a total of %d trace(s) were encountered during probing", len(tt.Traces)))

	for i, trace := range tt.Traces {
		slog.Debug(
			"trace:",
			"roundtrip", i,
			"init", trace.Init.UnixMilli(),
			"dnsExit", trace.DnsExit.UnixMilli(),
			"connectExit", trace.ConnectExit.UnixMilli(),
			"gotConnect", trace.GotConnect.UnixMilli(),
			"gotFirstByte", trace.GotFirstByte.UnixMilli(),
			"tlsEntry", trace.TlsEntry.UnixMilli(),
			"tlsExit", trace.TlsExit.UnixMilli(),
			"exit", trace.Exit.UnixMilli(),
		)

		if i != 0 {
			probeDurationGaugeVec.WithLabelValues("resolve").Add(trace.DnsExit.Sub(trace.Init).Seconds())
		}

		// continue here if we never got a connection because a request failed
		if trace.GotConnect.IsZero() {
			continue
		}

		if trace.Tls {
			probeDurationGaugeVec.WithLabelValues("connect").Add(trace.ConnectExit.Sub(trace.DnsExit).Seconds())
			probeDurationGaugeVec.WithLabelValues("tls").Add(trace.TlsExit.Sub(trace.TlsEntry).Seconds())
		} else {
			probeDurationGaugeVec.WithLabelValues("connect").Add(trace.GotConnect.Sub(trace.DnsExit).Seconds())
		}

		// continue here if we never got a response from the server
		if trace.GotFirstByte.IsZero() {
			continue
		}

		probeDurationGaugeVec.WithLabelValues("processing").Add(trace.GotFirstByte.Sub(trace.GotConnect).Seconds())

		// continue here if we never read the full response from the server
		// usually this means that request either failed or was redirected
		if trace.Exit.IsZero() {
			continue
		}

		probeDurationGaugeVec.WithLabelValues("transfer").Add(trace.Exit.Sub(trace.GotFirstByte).Seconds())
	}

	// *

	promHandler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	promHandler.ServeHTTP(ctx.Writer, ctx.Request)
}
