package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

type ShareViewerSession struct {
	ID        string
	IP        string
	UserAgent string
	LastSeen  time.Time
	CreatedAt time.Time
}

type ShareEntry struct {
	ID             string
	UUID           string
	Channel        string
	Password       string
	ExpiresAt      time.Time
	MaxConnections int
	Revoked        bool
	CreatedAt      time.Time
	ViewerSessions map[string]*ShareViewerSession
}

type ShareManager struct {
	mutex  sync.Mutex
	shares map[string]*ShareEntry
}

type ShareTokenPayload struct {
	ShareID  string `json:"share_id"`
	ViewerID string `json:"viewer_id"`
	UUID     string `json:"uuid"`
	Channel  string `json:"channel"`
	Exp      int64  `json:"exp"`
}

var ShareStore = NewShareManager()

func NewShareManager() *ShareManager {
	obj := &ShareManager{shares: make(map[string]*ShareEntry)}
	go obj.cleanupLoop()
	return obj
}

func (obj *ShareManager) cleanupLoop() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		obj.mutex.Lock()
		now := time.Now()
		for id, share := range obj.shares {
			obj.cleanupShareSessions(share, now)
			if now.After(share.ExpiresAt) {
				delete(obj.shares, id)
			}
		}
		obj.mutex.Unlock()
	}
}

func (obj *ShareManager) cleanupShareSessions(share *ShareEntry, now time.Time) {
	for viewerID, session := range share.ViewerSessions {
		if now.Sub(session.LastSeen) > 45*time.Second {
			delete(share.ViewerSessions, viewerID)
		}
	}
}

func (obj *ShareManager) Create(uuid string, channel string, expireMinutes int, maxConnections int) (*ShareEntry, error) {
	if expireMinutes <= 0 {
		expireMinutes = Storage.ServerShare().DefaultExpireMinutes
	}
	if expireMinutes <= 0 {
		expireMinutes = 60
	}
	if maxConnections < 1 || maxConnections > 5 {
		maxConnections = Storage.ServerShare().DefaultMaxConnections
	}
	if maxConnections < 1 || maxConnections > 5 {
		maxConnections = 1
	}
	entry := &ShareEntry{
		ID:             randomAlphaNum(12),
		UUID:           uuid,
		Channel:        channel,
		Password:       randomSharePassword(),
		ExpiresAt:      time.Now().Add(time.Duration(expireMinutes) * time.Minute),
		MaxConnections: maxConnections,
		CreatedAt:      time.Now(),
		ViewerSessions: make(map[string]*ShareViewerSession),
	}
	obj.mutex.Lock()
	obj.shares[entry.ID] = entry
	obj.mutex.Unlock()
	return entry, nil
}

func randomSharePassword() string {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	out := make([]byte, 4)
	raw := []byte(randomAlphaNum(8))
	for i := 0; i < 4; i++ {
		idx := int(raw[i]) % len(alphabet)
		out[i] = alphabet[idx]
	}
	return string(out)
}

func (obj *ShareManager) Get(shareID string) (*ShareEntry, error) {
	obj.mutex.Lock()
	defer obj.mutex.Unlock()
	entry, ok := obj.shares[shareID]
	if !ok {
		return nil, ErrorShareNotFound
	}
	if entry.Revoked {
		return nil, ErrorShareRevoked
	}
	if time.Now().After(entry.ExpiresAt) {
		delete(obj.shares, shareID)
		return nil, ErrorShareExpired
	}
	obj.cleanupShareSessions(entry, time.Now())
	return cloneShareEntry(entry), nil
}

func cloneShareEntry(entry *ShareEntry) *ShareEntry {
	if entry == nil {
		return nil
	}
	copyEntry := *entry
	copyEntry.ViewerSessions = make(map[string]*ShareViewerSession, len(entry.ViewerSessions))
	for key, val := range entry.ViewerSessions {
		sessionCopy := *val
		copyEntry.ViewerSessions[key] = &sessionCopy
	}
	return &copyEntry
}

func (obj *ShareManager) StartViewer(shareID string, password string, ip string, userAgent string) (string, string, *ShareEntry, error) {
	obj.mutex.Lock()
	defer obj.mutex.Unlock()

	entry, ok := obj.shares[shareID]
	if !ok {
		return "", "", nil, ErrorShareNotFound
	}
	if entry.Revoked {
		return "", "", nil, ErrorShareRevoked
	}
	now := time.Now()
	if now.After(entry.ExpiresAt) {
		delete(obj.shares, shareID)
		return "", "", nil, ErrorShareExpired
	}
	if strings.ToUpper(strings.TrimSpace(password)) != entry.Password {
		return "", "", nil, ErrorSharePasswordInvalid
	}
	obj.cleanupShareSessions(entry, now)
	if len(entry.ViewerSessions) >= entry.MaxConnections {
		return "", "", nil, ErrorShareConnectionLimit
	}
	viewerID := randomAlphaNum(16)
	entry.ViewerSessions[viewerID] = &ShareViewerSession{
		ID:        viewerID,
		IP:        ip,
		UserAgent: userAgent,
		CreatedAt: now,
		LastSeen:  now,
	}
	token, err := buildShareAccessToken(ShareTokenPayload{
		ShareID:  entry.ID,
		ViewerID: viewerID,
		UUID:     entry.UUID,
		Channel:  entry.Channel,
		Exp:      entry.ExpiresAt.Unix(),
	})
	if err != nil {
		delete(entry.ViewerSessions, viewerID)
		return "", "", nil, err
	}
	return viewerID, token, cloneShareEntry(entry), nil
}

func (obj *ShareManager) Heartbeat(shareID string, viewerID string) error {
	obj.mutex.Lock()
	defer obj.mutex.Unlock()
	entry, ok := obj.shares[shareID]
	if !ok {
		return ErrorShareNotFound
	}
	if entry.Revoked {
		return ErrorShareRevoked
	}
	now := time.Now()
	if now.After(entry.ExpiresAt) {
		delete(obj.shares, shareID)
		return ErrorShareExpired
	}
	session, ok := entry.ViewerSessions[viewerID]
	if !ok {
		return ErrorShareTokenInvalid
	}
	session.LastSeen = now
	return nil
}

func (obj *ShareManager) StopViewer(shareID string, viewerID string) {
	obj.mutex.Lock()
	defer obj.mutex.Unlock()
	if entry, ok := obj.shares[shareID]; ok {
		delete(entry.ViewerSessions, viewerID)
	}
}

func (obj *ShareManager) Revoke(shareID string) error {
	obj.mutex.Lock()
	defer obj.mutex.Unlock()
	entry, ok := obj.shares[shareID]
	if !ok {
		return ErrorShareNotFound
	}
	entry.Revoked = true
	entry.ViewerSessions = make(map[string]*ShareViewerSession)
	return nil
}

func (obj *ShareManager) Status(shareID string) (map[string]interface{}, error) {
	obj.mutex.Lock()
	defer obj.mutex.Unlock()
	entry, ok := obj.shares[shareID]
	if !ok {
		return nil, ErrorShareNotFound
	}
	obj.cleanupShareSessions(entry, time.Now())
	return map[string]interface{}{
		"id":                 entry.ID,
		"uuid":               entry.UUID,
		"channel":            entry.Channel,
		"expires_at":         entry.ExpiresAt.Unix(),
		"max_connections":    entry.MaxConnections,
		"active_connections": len(entry.ViewerSessions),
		"revoked":            entry.Revoked,
	}, nil
}

func buildShareAccessToken(payload ShareTokenPayload) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(body)
	signature := signShareToken(encoded)
	return fmt.Sprintf("%s.%s", encoded, signature), nil
}

func signShareToken(encoded string) string {
	secret := Storage.ServerShare().SignSecret
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(encoded))
	return hex.EncodeToString(mac.Sum(nil))
}

func parseShareAccessToken(token string) (*ShareTokenPayload, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return nil, ErrorShareTokenInvalid
	}
	if !hmac.Equal([]byte(signShareToken(parts[0])), []byte(parts[1])) {
		return nil, ErrorShareTokenInvalid
	}
	body, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, ErrorShareTokenInvalid
	}
	var payload ShareTokenPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, ErrorShareTokenInvalid
	}
	if payload.ShareID == "" || payload.ViewerID == "" {
		return nil, ErrorShareTokenInvalid
	}
	if time.Now().Unix() > payload.Exp {
		return nil, ErrorShareExpired
	}
	return &payload, nil
}

func (obj *ShareManager) ValidateAccessToken(token string, uuid string, channel string) (*ShareTokenPayload, error) {
	payload, err := parseShareAccessToken(token)
	if err != nil {
		return nil, err
	}
	if payload.UUID != uuid || payload.Channel != channel {
		return nil, ErrorShareTokenInvalid
	}

	obj.mutex.Lock()
	defer obj.mutex.Unlock()
	entry, ok := obj.shares[payload.ShareID]
	if !ok {
		return nil, ErrorShareNotFound
	}
	if entry.Revoked {
		return nil, ErrorShareRevoked
	}
	now := time.Now()
	if now.After(entry.ExpiresAt) {
		delete(obj.shares, payload.ShareID)
		return nil, ErrorShareExpired
	}
	obj.cleanupShareSessions(entry, now)
	session, ok := entry.ViewerSessions[payload.ViewerID]
	if !ok {
		return nil, ErrorShareTokenInvalid
	}
	session.LastSeen = now
	return payload, nil
}
