package stream

/*
"grpcSettings": {
	"serviceName": "",
	"multiMode": false,
	"idle_timeout": 60,
	"health_check_timeout": 20,
	"permit_without_stream": false,
	"initial_windows_size": 0
}
*/
// skip! go:generate gonstructor --type=GrpcConfig --constructorTypes=allArgs,builder --output=grpc_gen.go
type GrpcConfig struct {
	ServiceName         string `json:"serviceName"`
	MultiMode           bool   `json:"multiMode"`
	IdleTimeout         int    `json:"idle_timeout"`
	HealthCheckTimeout  int    `json:"health_check_timeout"`
	PermitWithoutStream bool   `json:"permit_without_stream"`
	InitialWindowsSize  int    `json:"initial_windows_size"`
}
