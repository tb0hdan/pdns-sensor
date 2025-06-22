package utils

import (
	"regexp"
	"strings"
)

func IsDomain(domain string) bool {
	reg := regexp.MustCompile(`(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z0-9][a-z0-9-]{0,61}[a-z0-9]`)
	return reg.MatchString(domain)
}

func IsValidDomain(domain string) bool {
	// Sanity checks for domain length and structure
	if len(domain) < 3 || len(domain) > 253 {
		return false
	}
	//
	if strings.HasSuffix(domain, ".local") || strings.HasSuffix(domain, ".localhost") {
		return false
	}
	// Check for dot
	parts := strings.Split(domain, ".")
	if len(parts) < 2 {
		return false
	}
	//
	return IsDomain(domain)
}
