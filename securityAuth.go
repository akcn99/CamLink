package main

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

const adminSessionCookieName = "rtsp_admin_session"

type AdminSession struct {
	Username  string
	ExpiresAt time.Time
}

type AdminSessionStore struct {
	mutex    sync.Mutex
	sessions map[string]AdminSession
}

var AdminSessions = NewAdminSessionStore()

func NewAdminSessionStore() *AdminSessionStore {
	obj := &AdminSessionStore{sessions: make(map[string]AdminSession)}
	go obj.cleanupLoop()
	return obj
}

func (obj *AdminSessionStore) cleanupLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		obj.mutex.Lock()
		for token, session := range obj.sessions {
			if now.After(session.ExpiresAt) {
				delete(obj.sessions, token)
			}
		}
		obj.mutex.Unlock()
	}
}

func (obj *AdminSessionStore) Create(username string, ttl time.Duration) (string, time.Time) {
	token := randomAlphaNum(48)
	expireAt := time.Now().Add(ttl)
	obj.mutex.Lock()
	obj.sessions[token] = AdminSession{Username: username, ExpiresAt: expireAt}
	obj.mutex.Unlock()
	return token, expireAt
}

func (obj *AdminSessionStore) Delete(token string) {
	obj.mutex.Lock()
	delete(obj.sessions, token)
	obj.mutex.Unlock()
}

func (obj *AdminSessionStore) Validate(token string) bool {
	obj.mutex.Lock()
	defer obj.mutex.Unlock()
	session, ok := obj.sessions[token]
	if !ok {
		return false
	}
	if time.Now().After(session.ExpiresAt) {
		delete(obj.sessions, token)
		return false
	}
	return true
}

func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func VerifyPassword(hash string, password string) bool {
	if hash == "" || password == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func randomAlphaNum(length int) string {
	if length <= 0 {
		return ""
	}
	raw := make([]byte, length)
	if _, err := rand.Read(raw); err != nil {
		return "fallback1234"
	}
	encoded := base64.RawURLEncoding.EncodeToString(raw)
	if len(encoded) > length {
		return encoded[:length]
	}
	return encoded
}

func ValidateAdminCredentials(username string, password string) bool {
	if username == "" || password == "" {
		return false
	}
	if username != Storage.ServerHTTPLogin() {
		return false
	}
	hash := Storage.ServerAdminPasswordHash()
	if hash != "" {
		return VerifyPassword(hash, password)
	}
	legacy := Storage.ServerHTTPPassword()
	return legacy != "" && legacy == password
}

func IsAdminRequestAuthorized(req *http.Request) bool {
	if req == nil {
		return false
	}
	if cookie, err := req.Cookie(adminSessionCookieName); err == nil {
		if AdminSessions.Validate(cookie.Value) {
			return true
		}
	}
	username, password, ok := req.BasicAuth()
	if ok && ValidateAdminCredentials(username, password) {
		return true
	}
	return false
}

func setAdminSessionCookie(c *gin.Context, token string, expireAt time.Time) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     adminSessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   c.Request.TLS != nil,
		SameSite: http.SameSiteLaxMode,
		Expires:  expireAt,
	})
}

func clearAdminSessionCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     adminSessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   c.Request.TLS != nil,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	})
}

func AdminAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if IsAdminRequestAuthorized(c.Request) {
			c.Next()
			return
		}
		path := c.Request.URL.Path
		if c.Request.Method != http.MethodGet || strings.HasPrefix(path, "/stream/") || strings.HasPrefix(path, "/streams") || strings.HasPrefix(path, "/settings") || strings.HasPrefix(path, "/share/") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, Message{Status: 0, Payload: ErrorUnauthorized.Error()})
			return
		}
		next := c.Request.URL.RequestURI()
		c.Redirect(http.StatusFound, "/login?next="+url.QueryEscape(next))
		c.Abort()
	}
}

type LoginPayload struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func HTTPAPILoginPage(c *gin.Context) {
	if IsAdminRequestAuthorized(c.Request) {
		c.Redirect(http.StatusFound, "/")
		return
	}
	c.HTML(http.StatusOK, "login.tmpl", gin.H{
		"page":    "login",
		"version": time.Now().String(),
		"ui_lang": Storage.ServerUILanguageDefault(),
	})
}

func HTTPAPIAuthLogin(c *gin.Context) {
	var payload LoginPayload
	if err := c.BindJSON(&payload); err != nil {
		c.IndentedJSON(http.StatusBadRequest, Message{Status: 0, Payload: err.Error()})
		return
	}
	if !ValidateAdminCredentials(payload.Username, payload.Password) {
		c.IndentedJSON(http.StatusUnauthorized, Message{Status: 0, Payload: ErrorInvalidCredentials.Error()})
		return
	}
	ttlMinutes := Storage.ServerSessionTTLMinutes()
	if ttlMinutes <= 0 {
		ttlMinutes = 720
	}
	token, expireAt := AdminSessions.Create(payload.Username, time.Duration(ttlMinutes)*time.Minute)
	setAdminSessionCookie(c, token, expireAt)
	c.IndentedJSON(http.StatusOK, Message{Status: 1, Payload: map[string]interface{}{
		"expires_at": expireAt.Unix(),
	}})
}

func HTTPAPIAuthLogout(c *gin.Context) {
	if cookie, err := c.Request.Cookie(adminSessionCookieName); err == nil {
		AdminSessions.Delete(cookie.Value)
	}
	clearAdminSessionCookie(c)
	c.IndentedJSON(http.StatusOK, Message{Status: 1, Payload: Success})
}
