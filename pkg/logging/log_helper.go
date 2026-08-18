package logging

import (
	"fmt"
	"time"
)

func ConvertMapToInterface(extra map[string]any) []any {
	var pairs []any
	for key, value := range extra {
		pairs = append(pairs, key, value)
	}
	return pairs
}

func GetLogFileNamePerDay(folderPath string) string {
	time := time.Now().Format("2006-01-02")
	return fmt.Sprintf("%s/app-%s.log", folderPath, time)
}
