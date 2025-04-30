package stackdepthbench

//go:noinline
func Call(a, b, c, d, e, f, g, h int64, depth int) int64 {
	if depth == 0 {
		return (123 + a - b) * (1 + c*d%e) * (f * g * h)
	}
	return Call(a, b, c, d, e, f, g, h, depth-1)
}
