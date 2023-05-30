package utils

import "regexp"

var (
	specialChars = regexp.MustCompile(`\W`)
)

func Sanitize(t string) string {
	return specialChars.ReplaceAllString(t, "_")
}
