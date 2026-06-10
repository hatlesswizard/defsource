package testdata

// Handler responds to an HTTP request.
//
// ServeHTTP should write reply headers and data to the ResponseWriter
// and then return. Returning signals that the request is finished.
type Handler interface {
	ServeHTTP(ResponseWriter, *Request)
}

// ReadWriter is the interface that groups the basic Read and Write methods.
type ReadWriter interface {
	Reader
	Writer
}

// Stringer is the interface implemented by any value that has a String method.
type Stringer interface {
	String() string
}
