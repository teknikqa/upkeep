package state

// ExportExpandHome exposes expandHome for testing.
func ExportExpandHome(path string) string {
	return expandHome(path)
}
