package serial

import (
	"cmp"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/gogf/gf/v2/encoding/gjson"
	"github.com/quyxishi/whitebox/internal/serial/xray"

	_ "github.com/xtls/xray-core/main/json"
)

type BackendType int

const (
	CONFIG_BACKEND_XRAYCORE BackendType = iota
	CONFIG_BACKEND_SINGBOX
)

type ParseParams struct {
	EnableDebug bool // {"loglevel":"debug","access":"none","error":""}
}

func ParseURI(backend BackendType, uri string, params *ParseParams) (out string, err error) {
	switch backend {
	case CONFIG_BACKEND_XRAYCORE:
		xrayConfig := xray.XrayConfig{}

		if params.EnableDebug {
			xrayConfig.Log = &xray.LogConfig{Loglevel: "debug", Access: "none", Error: ""}
		} else {
			xrayConfig.Log = &xray.LogConfig{Loglevel: "none"}
		}

		url, err := url.Parse(uri)
		if err != nil {
			log.Printf("[ERROR] serial/parse: unable to parse uri-based config as url.URL structure due: %v\n", err)
			return "", err
		}

		out, err = xrayConfig.Parse(url)
		if err != nil {
			log.Printf("[ERROR] serial/parse: unable to parse uri-based config for xray-core due: %v\n", err)
			return "", err
		}
	case CONFIG_BACKEND_SINGBOX:
		log.Panicln("[FATAL] serial/parse: uri-based config parser not implemented for 'sing-box' backend")
	default:
		log.Panicf("[FATAL] serial/parse: unexpected backend: %v\n", backend)
	}

	return out, nil
}

type ParseSubParams struct {
	EnableDebug  bool // {"loglevel":"debug","access":"none","error":""}
	FetchTimeout time.Duration
}

func ParseSubscriptionURI(json_sub_uri string, params *ParseSubParams) (out string, err error) {
	cli := http.Client{Timeout: cmp.Or(params.FetchTimeout, 5*time.Second)}
	resp, err := cli.Get(json_sub_uri)
	if err != nil {
		return "", fmt.Errorf("failed to fetch json subscription uri: %v", err)
	}
	if resp != nil {
		defer func() {
			if err := resp.Body.Close(); err != nil {
				log.Printf("[ERROR] serial/parse: failed to close response.body instance: %v", err)
			}
		}()
	}

	outRaw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read body from json subscription uri: %v", err)
	}

	if params.EnableDebug {
		debugFields := []struct {
			key string
			val any
		}{
			{"log.loglevel", "debug"},
			{"log.access", "none"},
			{"log.error", ""},
		}

		j := gjson.New(outRaw)

		for _, s := range debugFields {
			if err := j.Set(s.key, s.val); err != nil {
				return "", fmt.Errorf("failed to patch config with key '%s': %w", s.key, err)
			}
		}

		outRaw = []byte(j.MustToJsonString())
	}

	out = string(outRaw)
	return out, nil
}
