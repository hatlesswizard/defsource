package testdata

// Get is a wrapper around DefaultClient.Get.
func Get(url string) (*Response, error) {
	return DefaultClient.Get(url)
}

// ListenAndServe is a wrapper that delegates to the default server.
func ListenAndServe(addr string, handler Handler) error {
	return Serve(addr, handler)
}

// CacheGet wraps the cache access with error handling.
func CacheGet(key string) (Value, error) {
	v, err := cache.Get(key)
	if err != nil {
		return nil, err
	}
	return v, nil
}
