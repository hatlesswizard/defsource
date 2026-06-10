package testdata

// StatusOK indicates that the request succeeded.
const StatusOK = 200

// StatusNotFound indicates that the server cannot find the requested resource.
const StatusNotFound = 404

const (
	// MethodGet represents the GET HTTP method.
	MethodGet = "GET"

	// MethodPost represents the POST HTTP method.
	MethodPost = "POST"

	// MethodPut represents the PUT HTTP method.
	MethodPut = "PUT"
)

// MaxHeaderSize is the maximum allowed size for HTTP headers.
const MaxHeaderSize int = 1 << 20
