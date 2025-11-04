package conf

// AllowedHeaders defines the list of headers that are allowed to be passed through the gRPC-Gateway.
var AllowedHeaders = map[string]struct{}{
	"X-User-Id":     {},
	"X-User-Email":  {},
	"X-User-Name":   {},
	"X-User-Avatar": {},
}

func NewAllowedHeaders() map[string]struct{} {
	return AllowedHeaders
}
