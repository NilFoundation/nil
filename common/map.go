package common

func SliceToMap[T any, K comparable, V any](slice []T, mapper func(i int, t T) (K, V)) map[K]V {
	m := make(map[K]V, len(slice))
	for i, t := range slice {
		k, v := mapper(i, t)
		m[k] = v
	}
	return m
}
