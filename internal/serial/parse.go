package serial

import (
	"log"
	"net/url"

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
