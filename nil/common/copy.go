package common

func CopyPtr[T any](p *T) *T {
	tmp := *p
	return &tmp
}
