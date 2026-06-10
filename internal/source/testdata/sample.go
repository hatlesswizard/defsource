package sample

// Server represents an HTTP server with configurable options.
// It supports graceful shutdown and middleware chaining.
//
// Deprecated: Use NewServer instead.
type Server struct {
	// Addr is the address to listen on.
	Addr string
	// Handler is the HTTP handler.
	Handler interface{}
}

// NewServer creates a new Server instance with the given address.
func NewServer(addr string) *Server {
	return &Server{Addr: addr}
}

// ListenAndServe starts the server on the configured address.
func (s *Server) ListenAndServe() error {
	return nil
}
