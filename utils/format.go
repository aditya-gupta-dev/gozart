package utils

import "fmt"

var units = []string{"B", "KB", "MB", "GB", "TB", "PB"}

func FormatSize(size float64) string {
	unit := 0
	for size >= 1024.0 && unit < len(units)-1 {
		size /= 1024.0
		unit += 1
	}

	if unit == 0 {
		return fmt.Sprintf("%f %s", size, units[unit])
	} else {
		return fmt.Sprintf("%.2f %s", size, units[unit])
	}
}
