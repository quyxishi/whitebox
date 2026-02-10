package probe

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptrace"
	"net/textproto"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/cel-go/cel"
	"github.com/quyxishi/whitebox"
	"github.com/quyxishi/whitebox/internal/config"
	"github.com/quyxishi/whitebox/internal/serial"
	"golang.org/x/net/publicsuffix"

	"github.com/gin-gonic/gin"
	"github.com/gvcgo/vpnparser/pkgs/utils"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	_ "github.com/amnezia-vpn/amnezia-xray-core/app/dispatcher"
	_ "github.com/amnezia-vpn/amnezia-xray-core/app/proxyman/inbound"
	_ "github.com/amnezia-vpn/amnezia-xray-core/app/proxyman/outbound"
	"github.com/amnezia-vpn/amnezia-xray-core/common/net"
	"github.com/amnezia-vpn/amnezia-xray-core/core"
	_ "github.com/amnezia-vpn/amnezia-xray-core/main/json"
	_ "github.com/amnezia-vpn/amnezia-xray-core/proxy/blackhole"
	_ "github.com/amnezia-vpn/amnezia-xray-core/proxy/freedom"
	_ "github.com/amnezia-vpn/amnezia-xray-core/proxy/wireguard"
)

func matchRegularExpression(body []byte, predicate string) (bool, error) {
	regex, err := regexp.Compile(predicate)
	if err != nil {
		return false, fmt.Errorf("failed to compile regex:%s due: %v", predicate, err)
	}

	return regex.Match(body), nil
}

func newCELExpression(predicate string) (*cel.Program, error) {
	env, err := cel.NewEnv(
		cel.Variable("body", cel.DynType),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to construct CEL environment due: %v", err)
	}

	ast, issues := env.Compile(predicate)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("failed to compile CEL:%s due: %v", predicate, issues.Err())
	}

	celExpr, err := env.Program(ast, cel.InterruptCheckFrequency(100))
	if err != nil {
		return nil, fmt.Errorf("failed to construct CEL:%s due: %v", predicate, err)
	}

	return &celExpr, nil
}

func matchCELExpression(ctx context.Context, body []byte, predicate string) (bool, error) {
	var bodyJSON any
	if err := json.Unmarshal(body, &bodyJSON); err != nil {
		return false, fmt.Errorf("failed to unmarshall http body to json due: %v", err)
	}

	evalPayload := map[string]any{
		"body": bodyJSON,
	}

	celExpr, err := newCELExpression(predicate)
	if err != nil {
		return false, fmt.Errorf("unable to perform CEL validation due: %v", err)
	}

	result, details, err := (*celExpr).ContextEval(ctx, evalPayload)
	if err != nil {
		return false, fmt.Errorf("failed to evaluate CEL:%s due: %v", predicate, err)
	}
	if result.Type() != cel.BoolType {
		return false, fmt.Errorf("on CEL:%s evaluation result is not a boolean, details: %v", predicate, details)
	}

	return result.Value().(bool), nil
}

func matchRegularExpressionsOnHeaders(headers http.Header, key string, predicate string) (bool, error) {
	values := headers[textproto.CanonicalMIMEHeaderKey(key)]
	if len(values) == 0 {
		// currently, blindly treating missing header as validation fail
		return false, nil
	}

	regex, err := regexp.Compile(predicate)
	if err != nil {
		return false, fmt.Errorf("failed to compile regex:%s due: %v", predicate, err)
	}

	for _, v := range values {
		if !regex.MatchString(v) {
			return false, nil
		}
	}

	return true, nil
}

func matchStatusCodes(actual int, expecting []string) bool {
	matched := false

	for _, codeStr := range expecting {
		codeStr = strings.TrimSpace(codeStr)
		if strings.Contains(codeStr, "-") {
			// range match: "300-399"
			bounds := strings.Split(codeStr, "-")
			if len(bounds) == 2 {
				min, _ := strconv.Atoi(strings.TrimSpace(bounds[0]))
				max, _ := strconv.Atoi(strings.TrimSpace(bounds[1]))
				if actual >= min && actual <= max {
					matched = true
					break
				}
			}
		} else {
			// exact match: "200,404"
			code, _ := strconv.Atoi(codeStr)
			if actual == code {
				matched = true
				break
			}
		}
	}

	return matched
}

type ProbeHandler struct {
	configWrapper *config.WhiteboxConfigWrapper
}

func NewProbeHandler(wrapper *config.WhiteboxConfigWrapper) *ProbeHandler {
	return &ProbeHandler{
		configWrapper: wrapper,
	}
}

type ProbeParams struct {
	Connection string
	Scheme     string
	Target     string
	Scope      string
}

func (h *ProbeHandler) parseProbeParams(ctx *gin.Context, cfg *config.WhiteboxConfig) (out ProbeParams, ok bool) {
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

	out.Scope = ctx.DefaultQuery("scope", config.DefaultScopeName)
	if _, ok = cfg.Scopes[out.Scope]; !ok {
		ctx.String(http.StatusBadRequest, fmt.Sprintf("Scope '%s' does not exists in configuration", out.Scope))
		return
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

	cfg := h.configWrapper.Get()
	params, ok := h.parseProbeParams(ctx, cfg)
	if !ok {
		return
	}

	xrayConf, ok := h.parseXrayConf(ctx, &params)
	if !ok {
		return
	}

	scope := cfg.Scopes[params.Scope]

	slog.Info(
		"recv probe w/",
		"scheme", params.Scheme,
		"target", params.Target,
		"scope", params.Scope,
		"timeout", scope.Timeout,
		"http.method", scope.Http.Method,
		"http.maxRedirects", scope.Http.MaxRedirects,
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

	probeFailedDueToSSL := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tun_probe_failed_due_to_ssl",
		Help: "Indicates if probe failed due to ssl",
	})

	probeFailedDueToRegexBody := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tun_probe_failed_due_to_regex_body",
		Help: "Indicates if probe failed due to regex on body",
	})

	probeFailedDueToRegexHeader := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tun_probe_failed_due_to_regex_header",
		Help: "Indicates if probe failed due to regex on header",
	})

	probeFailedDueToCEL := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tun_probe_failed_due_to_cel",
		Help: "Indicates if probe failed due to CEL expression",
	})

	probeFailedDueToStatusCode := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tun_probe_failed_due_to_status",
		Help: "Indicates if probe failed due to status code",
	})

	failIfMetrics := map[config.FailIfModule]prometheus.Gauge{
		config.FailIf_SSL:                 probeFailedDueToSSL,
		config.FailIf_BodyMatchesRegexp:   probeFailedDueToRegexBody,
		config.FailIf_HeaderMatchesRegexp: probeFailedDueToRegexHeader,
		config.FailIf_BodyJsonMatchesCEL:  probeFailedDueToCEL,
		config.FailIf_StatusCodeMatches:   probeFailedDueToStatusCode,
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

	failIfBody := false
	for _, v := range scope.Http.FailIf {
		if !failIfBody && strings.HasPrefix(string(v.Mod), "body") {
			failIfBody = true
		}

		if metric, exists := failIfMetrics[v.Mod]; exists {
			registry.MustRegister(metric)
		}
	}

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

	redirectCounter := RedirectCounter{Max: scope.Http.MaxRedirects}

	client := &http.Client{
		Timeout: scope.Timeout,
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

	var body io.Reader

	if scope.Http.BodyFile != "" {
		fileData, err := os.ReadFile(scope.Http.BodyFile)
		if err != nil {
			slog.Error("failed to read body file", "path", scope.Http.BodyFile, "err", err)
			ctx.String(http.StatusInternalServerError, "Unable to read body file due: %v", err)
			return
		}

		body = bytes.NewReader(fileData)
	} else if scope.Http.Body != "" {
		body = strings.NewReader(scope.Http.Body)
	} else {
		body = bytes.NewBuffer([]byte{})
	}

	req, err := http.NewRequest(scope.Http.Method, params.Target, body)
	if err != nil {
		slog.Error("failed to to construct http.Request", "err", err)
		ctx.String(http.StatusInternalServerError, "Unable to construct http.Request due: %v", err)
		return
	}

	req.Header.Set("user-agent", fmt.Sprintf("whitebox/%s", whitebox.Version()))
	req.Header.Set("accept", "*/*")

	// non-canonical header keys assignment
	for k, v := range scope.Http.Headers {
		req.Header[k] = []string{v}
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

		var body []byte
		if failIfBody {
			body, err = io.ReadAll(byteCounter)
			if err != nil {
				slog.Error("failed to read http response body for validation", "err", err)
				probeSuccess = 0
			}
		}

		for i, failIf := range scope.Http.FailIf {
			var matched bool
			switch failIf.Mod {
			case config.FailIf_SSL:
				matched = (resp.TLS != nil)
				slog.Debug("failIf[SSL]: trace", "matched", matched, "inv", failIf.Inv)
			case config.FailIf_BodyMatchesRegexp:
				matched, err = matchRegularExpression(body, failIf.Val)
				slog.Debug("failIf[REGEXP_B]: trace", "matched", matched, "inv", failIf.Inv, "err", err)

				if err != nil {
					// force fail even if inv is false
					matched = !failIf.Inv
				}
			case config.FailIf_BodyJsonMatchesCEL:
				matched, err = matchCELExpression(ctx, body, failIf.Val)
				slog.Debug("failIf[CEL]: trace", "matched", matched, "inv", failIf.Inv, "err", err)

				if err != nil {
					matched = !failIf.Inv
				}
			case config.FailIf_HeaderMatchesRegexp:
				key, predicate, _ := strings.Cut(failIf.Val, ":")

				matched, err = matchRegularExpressionsOnHeaders(resp.Header, key, predicate)
				slog.Debug("failIf[REGEXP_H]: trace", "matched", matched, "inv", failIf.Inv, "err", err)

				if err != nil {
					matched = !failIf.Inv
				}
			case config.FailIf_StatusCodeMatches:
				statusCodes := strings.Split(failIf.Val, ",")

				matched = matchStatusCodes(resp.StatusCode, statusCodes)
				slog.Debug("failIf[STATUS]: trace", "matched", matched, "inv", failIf.Inv)
			default:
				slog.Warn("http.fail_if: unknown module", "idx", i, "mod", failIf.Mod)
				matched = !failIf.Inv
			}

			probeSuccess = float64(Bool2int(failIf.Inv != !matched))

			if probeSuccess == 0 {
				if metric, exists := failIfMetrics[failIf.Mod]; exists {
					metric.Set(1)
				}

				slog.Info("skipping future validations due already failed one")
				break
			}
		}

		if !failIfBody {
			// drain manually if we haven't already read it for validation
			_, err = io.Copy(io.Discard, byteCounter)
			if err != nil {
				slog.Error("failed to discard http response body", "err", err)
				probeSuccess = 0
			}
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
			"probe finished",
			"success", probeSuccess,
			"scheme", params.Scheme,
			"target", resp.Request.URL,
			"scope", params.Scope,
			"timeout", scope.Timeout,
			"http.method", resp.Request.Method,
			"http.redirects", redirectCounter.Total,
			"http.status", resp.Status,
			"http.contentLength", resp.ContentLength,
			"http.bodyLength", byteCounter.n,
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
