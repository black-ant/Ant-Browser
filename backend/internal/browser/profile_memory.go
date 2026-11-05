package browser

func normalizeMemoryLimitMB(value int) int {
	if value <= 0 {
		return 0
	}
	return value
}
