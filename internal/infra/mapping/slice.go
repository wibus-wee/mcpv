package mapping

func MapSlice[S any, D any](src []S, fn func(S) D) []D {
	dst := make([]D, 0, len(src))
	for _, item := range src {
		dst = append(dst, fn(item))
	}
	return dst
}
