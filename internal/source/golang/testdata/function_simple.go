package testdata

import "fmt"

// Println formats using the default formats for its operands and writes
// to standard output. Spaces are always added between operands and a
// newline is appended. It returns the number of bytes written and any
// write error encountered.
func Println(a ...interface{}) (n int, err error) {
	return fmt.Fprintln(nil, a...)
}

// HandleFunc registers the handler function for the given pattern.
func HandleFunc(pattern string, handler func(ResponseWriter, *Request)) {
	// implementation
}

// NewServer returns a new Server with the given address and handler.
func NewServer(addr string, handler Handler) *Server {
	return &Server{Addr: addr, Handler: handler}
}

// unexportedFunc should be skipped during discovery.
func unexportedFunc() {}
