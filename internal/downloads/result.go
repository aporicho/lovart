package downloads

// SuccessfulFileCount returns the number of file results without per-file errors.
func SuccessfulFileCount(files []FileResult) int {
	count := 0
	for _, file := range files {
		if file.Error == "" {
			count++
		}
	}
	return count
}
