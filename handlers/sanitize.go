package handlers

import (
	"regexp"
	"strings"
	"github.com/microcosm-cc/bluemonday"
)

var policy = bluemonday.StrictPolicy()

//Sanitasi teks biasa - hapus semua HTML
func SanitizeText(input string) string {
	//Trim Whitespace
	input = strings.TrimSpace(input)
	//Strip HTML Tags
	input = policy.Sanitize(input)
	return input
}

//Validasi slug kustom - hanya huruf, angka, dan tanda hubung
func IsValidSlug(slug string) bool {
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9-]+$`, slug)
	return matched
}

//Validasi URL - harus diawali http:// atu  https://
func IsValidURL(url string) bool {
	return strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")
}
