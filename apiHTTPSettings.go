package main

import (
	"net/url"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type GlobalSettingsPayload struct {
	UILanguageDefault string      `json:"ui_language_default"`
	PublicBaseURL     string      `json:"public_base_url"`
	Recording         RecordingST `json:"recording"`
	Share             ShareST     `json:"share"`
	HTTPLogin         string      `json:"http_login"`
	SessionTTLMinutes int         `json:"session_ttl_minutes"`
}

type AdminPasswordPayload struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

func HTTPAPISettingsPage(c *gin.Context) {
	c.HTML(http.StatusOK, "settings.tmpl", gin.H{
		"port":    Storage.ServerHTTPPort(),
		"streams": Storage.Streams,
		"version": time.Now().String(),
		"page":    "settings",
		"ui_lang": Storage.ServerUILanguageDefault(),
	})
}

func HTTPAPIGetGlobalSettings(c *gin.Context) {
	Storage.mutex.RLock()
	payload := GlobalSettingsPayload{
		UILanguageDefault: Storage.Server.UILanguageDefault,
		PublicBaseURL:     Storage.Server.PublicBaseURL,
		Recording:         Storage.Server.Recording,
		Share:             Storage.Server.Share,
		HTTPLogin:         Storage.Server.HTTPLogin,
		SessionTTLMinutes: Storage.Server.SessionTTLMinutes,
	}
	Storage.mutex.RUnlock()
	c.IndentedJSON(http.StatusOK, Message{Status: 1, Payload: payload})
}

func HTTPAPIUpdateGlobalSettings(c *gin.Context) {
	var payload GlobalSettingsPayload
	if err := c.BindJSON(&payload); err != nil {
		c.IndentedJSON(http.StatusBadRequest, Message{Status: 0, Payload: err.Error()})
		return
	}

	lang := strings.TrimSpace(payload.UILanguageDefault)
	if lang != "" && lang != "zh-CN" && lang != "en-US" {
		c.IndentedJSON(http.StatusBadRequest, Message{Status: 0, Payload: "unsupported ui language"})
		return
	}
	publicBaseURL := strings.TrimSpace(payload.PublicBaseURL)
	if publicBaseURL != "" {
		publicBaseURL = strings.TrimRight(publicBaseURL, "/")
		parsed, err := url.Parse(publicBaseURL)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			c.IndentedJSON(http.StatusBadRequest, Message{Status: 0, Payload: "invalid public base url"})
			return
		}
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			c.IndentedJSON(http.StatusBadRequest, Message{Status: 0, Payload: "public base url must start with http:// or https://"})
			return
		}
	}

	if payload.Recording.MaxDurationMinutes <= 0 {
		payload.Recording.MaxDurationMinutes = 60
	}
	if payload.Share.DefaultExpireMinutes <= 0 {
		payload.Share.DefaultExpireMinutes = 60
	}
	if payload.Share.DefaultMaxConnections < 1 || payload.Share.DefaultMaxConnections > 5 {
		payload.Share.DefaultMaxConnections = 1
	}
	if payload.Recording.Format == "" {
		payload.Recording.Format = "mp4"
	}
	if payload.Recording.FilenameRule == "" {
		payload.Recording.FilenameRule = "stream_channel_timestamp"
	}
	if payload.Recording.SavePath == "" {
		payload.Recording.SavePath = "save"
	}
	if payload.Recording.Resolution == "" {
		payload.Recording.Resolution = "source"
	}
	if payload.Recording.VideoCodec == "" {
		payload.Recording.VideoCodec = "source"
	}
	if payload.Share.SignSecret == "" {
		payload.Share.SignSecret = Storage.ServerShare().SignSecret
	}

	Storage.mutex.Lock()
	if lang != "" {
		Storage.Server.UILanguageDefault = lang
	}
	Storage.Server.PublicBaseURL = publicBaseURL
	Storage.Server.Recording = payload.Recording
	Storage.Server.Share = payload.Share
	if strings.TrimSpace(payload.HTTPLogin) != "" {
		Storage.Server.HTTPLogin = strings.TrimSpace(payload.HTTPLogin)
	}
	if payload.SessionTTLMinutes > 0 {
		Storage.Server.SessionTTLMinutes = payload.SessionTTLMinutes
	}
	err := Storage.SaveConfig()
	Storage.mutex.Unlock()
	if err != nil {
		c.IndentedJSON(http.StatusInternalServerError, Message{Status: 0, Payload: err.Error()})
		return
	}
	c.IndentedJSON(http.StatusOK, Message{Status: 1, Payload: Success})
}

func HTTPAPIUpdateAdminPassword(c *gin.Context) {
	var payload AdminPasswordPayload
	if err := c.BindJSON(&payload); err != nil {
		c.IndentedJSON(http.StatusBadRequest, Message{Status: 0, Payload: err.Error()})
		return
	}
	if len(payload.NewPassword) < 6 {
		c.IndentedJSON(http.StatusBadRequest, Message{Status: 0, Payload: "new password must be at least 6 characters"})
		return
	}
	if !ValidateAdminCredentials(Storage.ServerHTTPLogin(), payload.OldPassword) {
		c.IndentedJSON(http.StatusUnauthorized, Message{Status: 0, Payload: ErrorInvalidCredentials.Error()})
		return
	}

	hash, err := HashPassword(payload.NewPassword)
	if err != nil {
		c.IndentedJSON(http.StatusInternalServerError, Message{Status: 0, Payload: err.Error()})
		return
	}

	Storage.mutex.Lock()
	Storage.Server.AdminPasswordHash = hash
	Storage.Server.HTTPPassword = ""
	err = Storage.SaveConfig()
	Storage.mutex.Unlock()
	if err != nil {
		c.IndentedJSON(http.StatusInternalServerError, Message{Status: 0, Payload: err.Error()})
		return
	}
	c.IndentedJSON(http.StatusOK, Message{Status: 1, Payload: Success})
}
