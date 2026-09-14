package utils

import (
	"html"
	"strings"
)

func Santize(data string) string {
	data = html.EscapeString(strings.TrimSpace(data))
	return data
}
