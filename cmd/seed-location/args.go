package main

import (
	"strconv"
	"strings"
	"unicode"
)

// RepairSeedArgs undoes PowerShell + `go run --` damage:
//
//	-- -lat=10 .7486 -lng=106 .6601  →  -lat=10.7486 -lng=106.6601
//	-- -coords=  10.79,106.78       →  -coords=10.79,106.78
func RepairSeedArgs(args []string) []string {
	stripped := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "--" {
			continue
		}
		stripped = append(stripped, arg)
	}
	return mergeEmptyEquals(stitchDecimals(stripped))
}

func stitchDecimals(args []string) []string {
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		if i+1 < len(args) && endsWithDigit(args[i]) && isDecimalTail(args[i+1]) {
			out = append(out, args[i]+args[i+1])
			i++
			continue
		}
		out = append(out, args[i])
	}
	return out
}

func mergeEmptyEquals(args []string) []string {
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") && strings.HasSuffix(arg, "=") && i+1 < len(args) && !strings.HasPrefix(strings.TrimSpace(args[i+1]), "-") {
			out = append(out, arg+strings.TrimSpace(args[i+1]))
			i++
			continue
		}
		out = append(out, arg)
	}
	return out
}

func numericArgs(args []string) []string {
	out := make([]string, 0, len(args))
	for _, arg := range args {
		if strings.HasPrefix(strings.TrimSpace(arg), "-") {
			continue
		}
		out = append(out, arg)
	}
	return out
}

func endsWithDigit(s string) bool {
	if s == "" {
		return false
	}
	return unicode.IsDigit(rune(s[len(s)-1]))
}

func isDecimalTail(s string) bool {
	if !strings.HasPrefix(s, ".") {
		return false
	}
	_, err := strconv.ParseFloat("0"+s, 64)
	return err == nil
}
