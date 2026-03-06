package main

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type ShareCreatePayload struct {
	UUID           string `json:"uuid"`
	Channel        string `json:"channel"`
	ExpireMinutes  int    `json:"expire_minutes"`
	MaxConnections int    `json:"max_connections"`
}

type ShareSessionPayload struct {
	Password string `json:"password"`
	ViewerID string `json:"viewer_id"`
}

func HTTPAPIShareCreate(c *gin.Context) {
	var payload ShareCreatePayload
	if err := c.BindJSON(&payload); err != nil {
		c.IndentedJSON(http.StatusBadRequest, Message{Status: 0, Payload: err.Error()})
		return
	}
	if payload.Channel == "" {
		payload.Channel = "0"
	}
	if !Storage.StreamChannelExist(payload.UUID, payload.Channel) {
		c.IndentedJSON(http.StatusNotFound, Message{Status: 0, Payload: ErrorStreamNotFound.Error()})
		return
	}

	share, err := ShareStore.Create(payload.UUID, payload.Channel, payload.ExpireMinutes, payload.MaxConnections)
	if err != nil {
		c.IndentedJSON(http.StatusInternalServerError, Message{Status: 0, Payload: err.Error()})
		return
	}
	shareURL := buildAbsoluteURL(c, "/share/watch/"+share.ID+"?p="+url.QueryEscape(share.Password))
	qrURL := "https://api.qrserver.com/v1/create-qr-code/?size=240x240&data=" + url.QueryEscape(shareURL)
	c.IndentedJSON(http.StatusOK, Message{Status: 1, Payload: gin.H{
		"id":              share.ID,
		"password":        share.Password,
		"share_url":       shareURL,
		"qr_url":          qrURL,
		"expires_at":      share.ExpiresAt.Unix(),
		"max_connections": share.MaxConnections,
	}})
}

func HTTPAPIShareStatus(c *gin.Context) {
	status, err := ShareStore.Status(c.Param("id"))
	if err != nil {
		c.IndentedJSON(http.StatusNotFound, Message{Status: 0, Payload: err.Error()})
		return
	}
	c.IndentedJSON(http.StatusOK, Message{Status: 1, Payload: status})
}

func HTTPAPIShareRevoke(c *gin.Context) {
	if err := ShareStore.Revoke(c.Param("id")); err != nil {
		c.IndentedJSON(http.StatusNotFound, Message{Status: 0, Payload: err.Error()})
		return
	}
	c.IndentedJSON(http.StatusOK, Message{Status: 1, Payload: Success})
}

func HTTPAPIShareWatchPage(c *gin.Context) {
	shareID := c.Param("id")
	if _, err := ShareStore.Get(shareID); err != nil {
		c.String(http.StatusNotFound, err.Error())
		return
	}
	password := strings.ToUpper(strings.TrimSpace(c.Query("p")))
	c.HTML(http.StatusOK, "share_watch.tmpl", gin.H{
		"share_id":       shareID,
		"share_password": password,
		"version":        time.Now().String(),
		"page":           "share_watch",
		"ui_lang":        Storage.ServerUILanguageDefault(),
	})
}

func HTTPAPIShareSessionStart(c *gin.Context) {
	var payload ShareSessionPayload
	_ = c.BindJSON(&payload)
	password := strings.ToUpper(strings.TrimSpace(payload.Password))
	if password == "" {
		password = strings.ToUpper(strings.TrimSpace(c.Query("p")))
	}
	viewerID, token, share, err := ShareStore.StartViewer(c.Param("id"), password, c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		status := http.StatusBadRequest
		if err == ErrorShareConnectionLimit {
			status = http.StatusTooManyRequests
		}
		if err == ErrorShareNotFound {
			status = http.StatusNotFound
		}
		c.IndentedJSON(status, Message{Status: 0, Payload: err.Error()})
		return
	}
	c.IndentedJSON(http.StatusOK, Message{Status: 1, Payload: gin.H{
		"viewer_id":    viewerID,
		"share_token":  token,
		"uuid":         share.UUID,
		"channel":      share.Channel,
		"expires_at":   share.ExpiresAt.Unix(),
		"share_id":     share.ID,
		"active_count": len(share.ViewerSessions),
	}})
}

func HTTPAPIShareSessionHeartbeat(c *gin.Context) {
	var payload ShareSessionPayload
	if err := c.BindJSON(&payload); err != nil {
		c.IndentedJSON(http.StatusBadRequest, Message{Status: 0, Payload: err.Error()})
		return
	}
	if err := ShareStore.Heartbeat(c.Param("id"), payload.ViewerID); err != nil {
		c.IndentedJSON(http.StatusBadRequest, Message{Status: 0, Payload: err.Error()})
		return
	}
	c.IndentedJSON(http.StatusOK, Message{Status: 1, Payload: Success})
}

func HTTPAPIShareSessionStop(c *gin.Context) {
	var payload ShareSessionPayload
	_ = c.BindJSON(&payload)
	if payload.ViewerID != "" {
		ShareStore.StopViewer(c.Param("id"), payload.ViewerID)
	}
	c.IndentedJSON(http.StatusOK, Message{Status: 1, Payload: Success})
}

func buildAbsoluteURL(c *gin.Context, path string) string {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if baseURL := strings.TrimSpace(Storage.ServerPublicBaseURL()); baseURL != "" {
		return strings.TrimRight(baseURL, "/") + path
	}

	scheme := forwardedHeaderFirst(c.GetHeader("X-Forwarded-Proto"))
	if scheme == "" {
		scheme = forwardedHeaderFirst(c.GetHeader("X-Forwarded-Scheme"))
	}
	if scheme == "" {
		scheme = "http"
		if c.Request.TLS != nil {
			scheme = "https"
		}
	}
	host := forwardedHeaderFirst(c.GetHeader("X-Forwarded-Host"))
	if host == "" {
		host = c.Request.Host
	}
	if host == "" {
		host = "127.0.0.1:8083"
	}
	return scheme + "://" + host + path
}

func forwardedHeaderFirst(value string) string {
	if value == "" {
		return ""
	}
	parts := strings.Split(value, ",")
	if len(parts) == 0 {
		return ""
	}
	return strings.TrimSpace(parts[0])
}
