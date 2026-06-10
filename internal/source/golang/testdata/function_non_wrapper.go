package testdata

// Process is NOT a wrapper because it calls two functions.
func Process(x int) int {
	y := transform(x)
	return finalize(y)
}

// Handle is NOT a wrapper because it does not return a call result.
func Handle(w ResponseWriter, r *Request) {
	writeHeader(w)
	writeBody(w, r)
}

// Compute is NOT a wrapper because it has multiple return-influencing calls.
func Compute(a, b int) int {
	x := add(a, b)
	y := multiply(x, 2)
	return y
}
