package testdata

// ResponseWriter is an interface used by handlers.
type ResponseWriter interface {
	Write([]byte) (int, error)
	WriteHeader(statusCode int)
}

// Request represents an HTTP request.
type Request struct {
	Method string
	URL    *URL
}

// Client provides an HTTP client with embedded Transport.
type Client struct {
	*Transport
	Timeout int64
}

// Transport implements RoundTripper for HTTP.
type Transport struct {
	MaxIdleConns int
	IdleTimeout  int64
}

// Mux is a request multiplexer that embeds an interface.
type Mux struct {
	Handler
	routes map[string]Handler
}
