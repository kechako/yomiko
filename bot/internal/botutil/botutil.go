// Package botutil provides utility functions for working with bots.
package botutil

func PtrToValue[T any](p *T) T {
	var zero T
	return PtrToValueDefault(p, zero)
}

func PtrToValueDefault[T any](p *T, defaultValue T) T {
	if p == nil {
		return defaultValue
	}
	return *p
}
