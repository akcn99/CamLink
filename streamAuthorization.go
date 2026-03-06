package main

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

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
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		sep := "?"
		if strings.Contains(trimmed, "?") {
			sep = "&"
		}
		lines[i] = trimmed + sep + key + "=" + esc
	}
	return strings.Join(lines, "\n")
}
