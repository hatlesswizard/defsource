package testdata

// Server defines parameters for running an HTTP server.
// The zero value for Server is a valid configuration.
type Server struct {
	// Addr optionally specifies the TCP address for the server to listen on.
	Addr string

	// Handler specifies the handler to invoke.
	Handler Handler

	// ReadTimeout is the maximum duration for reading the entire request.
	ReadTimeout int64

	// WriteTimeout is the maximum duration before timing out writes.
	WriteTimeout int64

	// MaxHeaderBytes controls the maximum number of bytes the server will
	// read parsing the request header's keys and values.
	MaxHeaderBytes int

	unexportedField string
}

// ListenAndServe listens on the TCP network address srv.Addr and
// then calls Serve to handle requests on incoming connections.
func (srv *Server) ListenAndServe() error {
	return srv.serve()
}

// Shutdown gracefully shuts down the server without interrupting
// any active connections.
func (srv *Server) Shutdown(ctx Context) error {
	return nil
}

func (srv *Server) serve() error {
	return nil
}
