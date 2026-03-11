package main

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type DetectionServiceStatusPayload struct {
	ConfiguredURL string                    `json:"configured_url"`
	Reachable     bool                      `json:"reachable"`
	StatusCode    int                       `json:"status_code"`
	Message       string                    `json:"message"`
	EventStore    DetectionEventStoreStatus `json:"event_store"`
}

func HTTPAPIDetectionPage(c *gin.Context) {
	c.HTML(http.StatusOK, "detection.tmpl", gin.H{
		"port":            Storage.ServerHTTPPort(),
		"streams":         Storage.Streams,
		"version":         time.Now().String(),
		"page":            "detection",
		"ui_lang":         Storage.ServerUILanguageDefault(),
		"initial_uuid":    c.Query("uuid"),
		"initial_channel": c.DefaultQuery("channel", "0"),
	})
}

func HTTPAPIGetDetectionSettings(c *gin.Context) {
	c.IndentedJSON(http.StatusOK, Message{Status: 1, Payload: Storage.ServerDetection()})
}

func HTTPAPIUpdateDetectionSettings(c *gin.Context) {
	var payload DetectionServerST
	if err := c.BindJSON(&payload); err != nil {
		c.IndentedJSON(http.StatusBadRequest, Message{Status: 0, Payload: err.Error()})
		return
	}
	payload.DetectorURL = strings.TrimRight(strings.TrimSpace(payload.DetectorURL), "/")
	payload.EventsDBPath = strings.TrimSpace(payload.EventsDBPath)
	payload.ExportDir = strings.TrimSpace(payload.ExportDir)
	payload.AccessToken = strings.TrimSpace(payload.AccessToken)
	if payload.AccessToken == "" {
		payload.AccessToken = Storage.ServerDetection().AccessToken
	}
	if payload.DetectorURL != "" {
		parsed, err := url.Parse(payload.DetectorURL)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			c.IndentedJSON(http.StatusBadRequest, Message{Status: 0, Payload: "invalid detector url"})
			return
		}
	}
	if err := Storage.DetectionSettingsUpdate(payload); err != nil {
		c.IndentedJSON(http.StatusInternalServerError, Message{Status: 0, Payload: err.Error()})
		return
	}
	current := Storage.ServerDetection()
	if err := DetectionEvents.Init(current.EventsDBPath, current.ExportDir); err != nil {
		c.IndentedJSON(http.StatusInternalServerError, Message{Status: 0, Payload: err.Error()})
		return
	}
	c.IndentedJSON(http.StatusOK, Message{Status: 1, Payload: Success})
}

func HTTPAPIGetDetectionOverview(c *gin.Context) {
	c.IndentedJSON(http.StatusOK, Message{Status: 1, Payload: Storage.DetectionOverview()})
}

func HTTPAPIGetDetectionConfig(c *gin.Context) {
	detection, err := Storage.DetectionChannelConfig(c.Param("uuid"), c.Param("channel"))
	if err != nil {
		c.IndentedJSON(http.StatusNotFound, Message{Status: 0, Payload: err.Error()})
		return
	}
	info, err := Storage.StreamInfo(c.Param("uuid"))
	if err != nil {
		c.IndentedJSON(http.StatusNotFound, Message{Status: 0, Payload: err.Error()})
		return
	}
	channel := info.Channels[c.Param("channel")]
	c.IndentedJSON(http.StatusOK, Message{Status: 1, Payload: gin.H{
		"stream_uuid": c.Param("uuid"),
		"stream_name": info.Name,
		"channel_id":  c.Param("channel"),
		"stream_url":  maskSensitiveStreamURL(channel.URL),
		"on_demand":   channel.OnDemand,
		"detection":   detection,
	}})
}

func maskSensitiveStreamURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if parsed.User == nil {
		return raw
	}
	username := parsed.User.Username()
	if username == "" {
		return raw
	}
	if _, ok := parsed.User.Password(); ok {
		parsed.User = url.UserPassword(username, "***")
		return parsed.String()
	}
	parsed.User = url.User(username)
	return parsed.String()
}

func HTTPAPIUpdateDetectionConfig(c *gin.Context) {
	var payload DetectionChannelST
	if err := c.BindJSON(&payload); err != nil {
		c.IndentedJSON(http.StatusBadRequest, Message{Status: 0, Payload: err.Error()})
		return
	}
	payload.Mode = strings.TrimSpace(payload.Mode)
	payload.TelegramChatID = strings.TrimSpace(payload.TelegramChatID)
	if payload.Mode == "" {
		payload.Mode = "vehicle_entry"
	}
	if payload.Mode != "vehicle_entry" {
		c.IndentedJSON(http.StatusBadRequest, Message{Status: 0, Payload: "unsupported detection mode"})
		return
	}
	payload.EntryDirection = strings.TrimSpace(strings.ToLower(payload.EntryDirection))
	if payload.EntryDirection == "" {
		payload.EntryDirection = "any"
	}
	allowedDirections := map[string]bool{
		"any":            true,
		"left_to_right":  true,
		"right_to_left":  true,
		"top_to_bottom":  true,
		"bottom_to_top":  true,
	}
	if !allowedDirections[payload.EntryDirection] {
		c.IndentedJSON(http.StatusBadRequest, Message{Status: 0, Payload: "unsupported entry_direction"})
		return
	}
	if payload.ConfidenceThreshold <= 0 {
		payload.ConfidenceThreshold = 0.35
	}
	if payload.SampleFPS <= 0 || payload.SampleFPS > 5 {
		c.IndentedJSON(http.StatusBadRequest, Message{Status: 0, Payload: "sample_fps must be between 1 and 5"})
		return
	}
	if payload.CooldownSeconds <= 0 || payload.CooldownSeconds > 3600 {
		c.IndentedJSON(http.StatusBadRequest, Message{Status: 0, Payload: "cooldown_seconds must be between 1 and 3600"})
		return
	}
	if payload.ConfidenceThreshold < 0.05 || payload.ConfidenceThreshold > 0.95 {
		c.IndentedJSON(http.StatusBadRequest, Message{Status: 0, Payload: "confidence_threshold must be between 0.05 and 0.95"})
		return
	}
	if payload.MinBoxArea < 0 || payload.MinBoxArea > 5000000 {
		c.IndentedJSON(http.StatusBadRequest, Message{Status: 0, Payload: "min_box_area must be between 0 and 5000000"})
		return
	}
	if payload.MinMovePixels < 0 || payload.MinMovePixels > 1000 {
		c.IndentedJSON(http.StatusBadRequest, Message{Status: 0, Payload: "min_move_px must be between 0 and 1000"})
		return
	}
	if payload.TriggerConsecutiveFrames <= 0 || payload.TriggerConsecutiveFrames > 10 {
		c.IndentedJSON(http.StatusBadRequest, Message{Status: 0, Payload: "trigger_consecutive_frames must be between 1 and 10"})
		return
	}
	if len(payload.Polygon) != 0 && len(payload.Polygon) < 3 {
		c.IndentedJSON(http.StatusBadRequest, Message{Status: 0, Payload: "polygon must contain at least 3 points"})
		return
	}
	for _, pt := range payload.Polygon {
		if math.IsNaN(pt.X) || math.IsNaN(pt.Y) || math.IsInf(pt.X, 0) || math.IsInf(pt.Y, 0) {
			c.IndentedJSON(http.StatusBadRequest, Message{Status: 0, Payload: "polygon contains invalid coordinates"})
			return
		}
		if pt.X < 0 || pt.Y < 0 {
			c.IndentedJSON(http.StatusBadRequest, Message{Status: 0, Payload: "polygon coordinates must be non-negative"})
			return
		}
	}
	allowedClasses := map[string]bool{"car": true, "motorcycle": true, "bicycle": true}
	classes := make([]string, 0, len(payload.Classes))
	seen := make(map[string]bool)
	for _, item := range payload.Classes {
		name := strings.ToLower(strings.TrimSpace(item))
		if name == "" || seen[name] {
			continue
		}
		if !allowedClasses[name] {
			c.IndentedJSON(http.StatusBadRequest, Message{Status: 0, Payload: fmt.Sprintf("unsupported class: %s", name)})
			return
		}
		seen[name] = true
		classes = append(classes, name)
	}
	payload.Classes = classes
	if payload.Enabled && payload.TelegramEnabled && payload.TelegramChatID == "" {
		c.IndentedJSON(http.StatusBadRequest, Message{Status: 0, Payload: "telegram_chat_id is required when telegram is enabled"})
		return
	}
	if err := Storage.DetectionChannelUpdate(c.Param("uuid"), c.Param("channel"), payload); err != nil {
		c.IndentedJSON(http.StatusInternalServerError, Message{Status: 0, Payload: err.Error()})
		return
	}
	c.IndentedJSON(http.StatusOK, Message{Status: 1, Payload: Success})
}

func detectorTokenAuthorized(c *gin.Context) bool {
	token := strings.TrimSpace(c.GetHeader("X-CamLink-Detector-Token"))
	if token == "" {
		token = strings.TrimSpace(c.Query("token"))
	}
	return token != "" && token == Storage.ServerDetection().AccessToken
}

func HTTPAPIDetectorConfig(c *gin.Context) {
	if !detectorTokenAuthorized(c) {
		c.AbortWithStatusJSON(http.StatusUnauthorized, Message{Status: 0, Payload: ErrorUnauthorized.Error()})
		return
	}
	items := make([]gin.H, 0)
	Storage.mutex.RLock()
	for streamUUID, stream := range Storage.Streams {
		for channelID, channel := range stream.Channels {
			if !channel.Detection.Enabled {
				continue
			}
			items = append(items, gin.H{
				"stream_uuid": streamUUID,
				"stream_name": stream.Name,
				"channel_id":  channelID,
				"stream_url":  channel.URL,
				"on_demand":   channel.OnDemand,
				"detection":   channel.Detection,
			})
		}
	}
	Storage.mutex.RUnlock()
	c.IndentedJSON(http.StatusOK, Message{Status: 1, Payload: gin.H{
		"items":       items,
		"server_time": time.Now().UTC().Format(time.RFC3339),
	}})
}

func HTTPAPIDetectorEventIngest(c *gin.Context) {
	if !detectorTokenAuthorized(c) {
		c.AbortWithStatusJSON(http.StatusUnauthorized, Message{Status: 0, Payload: ErrorUnauthorized.Error()})
		return
	}
	var payload DetectionEventST
	if err := c.BindJSON(&payload); err != nil {
		c.IndentedJSON(http.StatusBadRequest, Message{Status: 0, Payload: err.Error()})
		return
	}
	if err := hydrateDetectionEvent(&payload); err != nil {
		c.IndentedJSON(http.StatusBadRequest, Message{Status: 0, Payload: err.Error()})
		return
	}
	event, err := DetectionEvents.Append(payload)
	if err != nil {
		c.IndentedJSON(http.StatusInternalServerError, Message{Status: 0, Payload: err.Error()})
		return
	}
	c.IndentedJSON(http.StatusOK, Message{Status: 1, Payload: event})
}

func HTTPAPIGetDetectionEvents(c *gin.Context) {
	query, err := parseDetectionEventQuery(c)
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, Message{Status: 0, Payload: err.Error()})
		return
	}
	events, summary, err := DetectionEvents.List(query)
	if err != nil {
		c.IndentedJSON(http.StatusInternalServerError, Message{Status: 0, Payload: err.Error()})
		return
	}
	c.IndentedJSON(http.StatusOK, Message{Status: 1, Payload: gin.H{
		"events":  events,
		"summary": summary,
		"count":   len(events),
		"query": gin.H{
			"stream_uuid":  query.StreamUUID,
			"channel_id":   query.ChannelID,
			"object_class": query.ObjectClass,
		},
	}})
}

func HTTPAPIExportDetectionEvents(c *gin.Context) {
	query, err := parseDetectionEventQuery(c)
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, Message{Status: 0, Payload: err.Error()})
		return
	}
	buf, exportPath, err := DetectionEvents.ExportCSV(query)
	if err != nil {
		c.IndentedJSON(http.StatusInternalServerError, Message{Status: 0, Payload: err.Error()})
		return
	}
	filename := fmt.Sprintf("vehicle-entry-report-%s.csv", time.Now().Format("20060102-150405"))
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	if exportPath != "" {
		c.Header("X-CamLink-Export-File", exportPath)
	}
	_, _ = c.Writer.Write(buf)
}

func HTTPAPIIngestDetectionEvent(c *gin.Context) {
	var payload DetectionEventST
	if err := c.BindJSON(&payload); err != nil {
		c.IndentedJSON(http.StatusBadRequest, Message{Status: 0, Payload: err.Error()})
		return
	}
	if err := hydrateDetectionEvent(&payload); err != nil {
		c.IndentedJSON(http.StatusBadRequest, Message{Status: 0, Payload: err.Error()})
		return
	}
	event, err := DetectionEvents.Append(payload)
	if err != nil {
		c.IndentedJSON(http.StatusInternalServerError, Message{Status: 0, Payload: err.Error()})
		return
	}
	c.IndentedJSON(http.StatusOK, Message{Status: 1, Payload: event})
}

func HTTPAPICreateDetectionMockEvent(c *gin.Context) {
	streamUUID := strings.TrimSpace(c.Param("uuid"))
	channelID := strings.TrimSpace(c.Param("channel"))
	if streamUUID == "" || channelID == "" {
		c.IndentedJSON(http.StatusBadRequest, Message{Status: 0, Payload: "stream uuid and channel are required"})
		return
	}
	info, err := Storage.StreamInfo(streamUUID)
	if err != nil {
		c.IndentedJSON(http.StatusNotFound, Message{Status: 0, Payload: err.Error()})
		return
	}
	channel, ok := info.Channels[channelID]
	if !ok {
		c.IndentedJSON(http.StatusNotFound, Message{Status: 0, Payload: ErrorStreamChannelNotFound.Error()})
		return
	}
	objectClass := "motorcycle"
	if len(channel.Detection.Classes) != 0 {
		objectClass = channel.Detection.Classes[0]
	}
	event, err := DetectionEvents.Append(DetectionEventST{
		StreamUUID:  streamUUID,
		StreamName:  info.Name,
		ChannelID:   channelID,
		ObjectClass: objectClass,
		TrackID:     "mock-" + randomAlphaNum(8),
		EnteredAt:   time.Now(),
	})
	if err != nil {
		c.IndentedJSON(http.StatusInternalServerError, Message{Status: 0, Payload: err.Error()})
		return
	}
	c.IndentedJSON(http.StatusOK, Message{Status: 1, Payload: event})
}

func HTTPAPIGetDetectionServiceStatus(c *gin.Context) {
	payload := DetectionServiceStatusPayload{
		ConfiguredURL: Storage.ServerDetection().DetectorURL,
		Message:       "detector url not configured",
		EventStore:    DetectionEvents.Status(),
	}
	if payload.ConfiguredURL == "" {
		c.IndentedJSON(http.StatusOK, Message{Status: 1, Payload: payload})
		return
	}
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(payload.ConfiguredURL + "/healthz")
	if err != nil {
		payload.Message = err.Error()
		c.IndentedJSON(http.StatusOK, Message{Status: 1, Payload: payload})
		return
	}
	defer resp.Body.Close()
	payload.Reachable = resp.StatusCode >= 200 && resp.StatusCode < 300
	payload.StatusCode = resp.StatusCode
	payload.Message = resp.Status
	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err == nil {
		if msg, ok := body["message"].(string); ok && msg != "" {
			payload.Message = msg
		}
	}
	c.IndentedJSON(http.StatusOK, Message{Status: 1, Payload: payload})
}

func hydrateDetectionEvent(event *DetectionEventST) error {
	if event == nil {
		return fmt.Errorf("event payload is required")
	}
	event.StreamUUID = strings.TrimSpace(event.StreamUUID)
	event.ChannelID = strings.TrimSpace(event.ChannelID)
	if event.StreamUUID == "" || event.ChannelID == "" {
		return fmt.Errorf("stream_uuid and channel_id are required")
	}
	info, err := Storage.StreamInfo(event.StreamUUID)
	if err != nil {
		return err
	}
	channel, ok := info.Channels[event.ChannelID]
	if !ok {
		return ErrorStreamChannelNotFound
	}
	if strings.TrimSpace(event.StreamName) == "" {
		event.StreamName = info.Name
	}
	if strings.TrimSpace(event.ObjectClass) == "" {
		if len(channel.Detection.Classes) == 0 {
			return fmt.Errorf("object_class is required")
		}
		event.ObjectClass = channel.Detection.Classes[0]
	}
	return nil
}

func parseDetectionEventQuery(c *gin.Context) (DetectionEventQuery, error) {
	startedAfter, err := parseFlexibleTime(c.Query("start"))
	if err != nil {
		return DetectionEventQuery{}, fmt.Errorf("invalid start time")
	}
	startedBefore, err := parseFlexibleTime(c.Query("end"))
	if err != nil {
		return DetectionEventQuery{}, fmt.Errorf("invalid end time")
	}
	return DetectionEventQuery{
		StreamUUID:    strings.TrimSpace(c.Query("stream_uuid")),
		ChannelID:     strings.TrimSpace(c.Query("channel_id")),
		ObjectClass:   strings.TrimSpace(c.Query("object_class")),
		StartedAfter:  startedAfter,
		StartedBefore: startedBefore,
	}, nil
}

func parseFlexibleTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed, nil
	}
	layouts := []string{"2006-01-02T15:04", "2006-01-02 15:04:05", "2006-01-02"}
	for _, layout := range layouts {
		if parsed, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid time")
}
