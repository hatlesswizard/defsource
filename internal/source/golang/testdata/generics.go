package testdata

// Set is a generic set data structure.
type Set[T comparable] struct {
	items map[T]struct{}
}

// Pair holds two values of potentially different types.
type Pair[A, B any] struct {
	First  A
	Second B
}

// Map applies a function to each element of a slice and returns the results.
func Map[T, U any](s []T, f func(T) U) []U {
	result := make([]U, len(s))
	for i, v := range s {
		result[i] = f(v)
	}
	return result
}

// Contains checks if a value exists in a slice.
func Contains[T comparable](s []T, v T) bool {
	for _, item := range s {
		if item == v {
			return true
		}
	}
	return false
}

// Constraint is a generic interface with type constraints.
type Constraint[T int | float64 | string] interface {
	Value() T
}
