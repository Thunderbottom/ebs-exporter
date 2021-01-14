package main

import (
	"regexp"
)

// replaceWithUnderscores replaces special characters with
// underscores for prometheus metric naming convention:
// https://prometheus.io/docs/instrumenting/writing_exporters/#naming
func replaceWithUnderscores(text string) string {
	re := regexp.MustCompile("[^a-zA-Z0-9:_]+")
	return re.ReplaceAllString(text, "_")
}
