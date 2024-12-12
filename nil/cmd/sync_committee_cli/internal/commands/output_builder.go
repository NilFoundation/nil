package commands

import (
	"strings"

	"github.com/fatih/color"
)

type outputBuilder struct {
	strings.Builder
}

func (b *outputBuilder) WriteLine(parts ...string) {
	for _, part := range parts {
		b.WriteString(part)
	}
	b.WriteString("\n")
}

func GreenStr(format string, args ...any) string {
	return color.HiGreenString(format, args...)
}

func CyanStr(format string, args ...any) string {
	return color.HiCyanString(format, args...)
}

func YellowStr(format string, args ...any) string {
	return color.HiYellowString(format, args...)
}

func RedStr(format string, args ...any) string {
	return color.HiRedString(format, args...)
}
