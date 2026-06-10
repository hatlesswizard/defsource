package testdata

// HandlerFunc is an adapter to allow the use of ordinary functions as HTTP handlers.
type HandlerFunc func(ResponseWriter, *Request)

// Duration represents the elapsed time between two instants.
//
// Deprecated: Use time.Duration from the standard library instead.
type Duration = int64

// ByteSlice is a named type for a byte slice.
type ByteSlice []byte
