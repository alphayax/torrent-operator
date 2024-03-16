package helper

import "fmt"

// HumanReadableSize Convert a size in bytes to a human-readable size
func HumanReadableSize(sizeInit int64) string {
	// Handle zero size as a special case
	if sizeInit == 0 {
		return "0 B"
	}

	units := []string{"B", "KB", "MB", "GB", "TB", "PB", "EB"}
	size := float64(sizeInit)

	// Use float division for more accurate calculations
	index := 0
	for size >= 1024 && index < len(units)-1 {
		size = float64(size) / 1024
		index++
	}

	// Round the value to one decimal place and format output
	return fmt.Sprintf("%.3f %s", size, units[index])
}
