package utils

import (
	"net/url"
)

func Absolute(base, href string) string {
	u, err := url.Parse(href)
	if err != nil || href == "" {
		return href
	}
	if u.IsAbs() {
		return u.String()
	}
	if base == "" {
		return href
	}
	bu, err := url.Parse(base)
	if err != nil {
		return href
	}
	return bu.ResolveReference(u).String()
}
