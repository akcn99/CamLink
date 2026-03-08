package main

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

var m3u8URIAttrRegexp = regexp.MustCompile(`URI="([^"]+)"`)

func AuthorizeStreamAccess(c *gin.Context, proto string, stream string, channel string) bool {
	if IsAdminRequestAuthorized(c.Request) {
		return true
	}
	shareToken := c.Query("share_token")
	if shareToken != "" {
		if _, err := ShareStore.ValidateAccessToken(shareToken, stream, channel); err == nil {
			return true
		}
		c.AbortWithStatus(http.StatusUnauthorized)
		return false
	}
	if Storage.ServerTokenEnable() {
		if RemoteAuthorization(proto, stream, channel, c.Query("token"), c.ClientIP()) {
			return true
		}
		c.AbortWithStatus(http.StatusUnauthorized)
		return false
	}
	c.AbortWithStatus(http.StatusUnauthorized)
	return false
}

func AppendQueryToM3U8(index string, key string, value string) string {
	if index == "" || key == "" || value == "" {
		return index
	}
	esc := url.QueryEscape(value)
	lines := strings.Split(index, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			if strings.Contains(trimmed, "URI=\"") {
				lines[i] = m3u8URIAttrRegexp.ReplaceAllStringFunc(line, func(match string) string {
					uri := m3u8URIAttrRegexp.ReplaceAllString(match, `$1`)
					return `URI="` + appendQueryValue(uri, key, esc) + `"`
				})
			}
			continue
		}
		lines[i] = appendQueryValue(trimmed, key, esc)
	}
	return strings.Join(lines, "\n")
}

func appendQueryValue(raw string, key string, escapedValue string) string {
	sep := "?"
	if strings.Contains(raw, "?") {
		sep = "&"
	}
	return raw + sep + key + "=" + escapedValue
}
