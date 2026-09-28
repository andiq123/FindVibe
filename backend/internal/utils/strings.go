package utils

import "strings"

func NormalizeString(s string) string {
	s = strings.ToLower(s)
	s = strings.TrimSpace(s)
	s = strings.Join(strings.Fields(s), " ")
	return s
}

// UpgradeHTTPS normalizes http:// and protocol-relative URLs to https://.
func UpgradeHTTPS(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "http://") {
		return "https://" + strings.TrimPrefix(raw, "http://")
	}
	if strings.HasPrefix(raw, "//") {
		return "https:" + raw
	}
	return raw
}
