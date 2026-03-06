package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/deepch/vdk/format/mp4"
)

type RecordingSession struct {
	ID        string
	UUID      string
	Channel   string
	FilePath  string
	StartedAt time.Time
	stopCh    chan struct{}
	doneCh    chan struct{}
	Err       string
}

type RecordingManager struct {
	mutex    sync.Mutex
	sessions map[string]*RecordingSession
}

var Recorder = NewRecordingManager()

func NewRecordingManager() *RecordingManager {
	return &RecordingManager{
		sessions: make(map[string]*RecordingSession),
	}
}

func recordingKey(uuid string, channel string) string {
	return uuid + ":" + channel
}

func (obj *RecordingManager) Start(uuid string, channel string) (*RecordingSession, error) {
	obj.mutex.Lock()
	key := recordingKey(uuid, channel)
	if _, ok := obj.sessions[key]; ok {
		obj.mutex.Unlock()
		return nil, ErrorRecordingAlreadyRunning
	}
	cfg := Storage.ServerRecording()
	filePath, err := buildRecordingFilePath(cfg, uuid, channel)
	if err != nil {
		obj.mutex.Unlock()
		return nil, err
	}
	session := &RecordingSession{
		ID:        randomAlphaNum(14),
		UUID:      uuid,
		Channel:   channel,
		FilePath:  filePath,
		StartedAt: time.Now(),
		stopCh:    make(chan struct{}),
		doneCh:    make(chan struct{}),
	}
	obj.sessions[key] = session
	obj.mutex.Unlock()

	go obj.run(session, cfg, filePath)
	return session, nil
}

func buildRecordingFilePath(cfg RecordingST, uuid string, channel string) (string, error) {
	root := strings.TrimSpace(cfg.SavePath)
	if root == "" {
		root = "save"
	}
	if cfg.Format == "" {
		cfg.Format = "mp4"
	}
	if err := os.MkdirAll(root, 0755); err != nil {
		return "", err
	}
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("%s_ch%s_%s.%s", uuid, channel, timestamp, cfg.Format)
	return filepath.Join(root, filename), nil
}

func (obj *RecordingManager) run(session *RecordingSession, cfg RecordingST, path string) {
	defer close(session.doneCh)

	f, err := os.Create(path)
	if err != nil {
		obj.stopWithError(session, err.Error())
		return
	}
	defer f.Close()

	Storage.StreamChannelRun(session.UUID, session.Channel)
	cid, ch, _, err := Storage.ClientAdd(session.UUID, session.Channel, MSE)
	if err != nil {
		obj.stopWithError(session, err.Error())
		return
	}
	defer Storage.ClientDelete(session.UUID, cid, session.Channel)

	codecs, err := Storage.StreamChannelCodecs(session.UUID, session.Channel)
	if err != nil {
		obj.stopWithError(session, err.Error())
		return
	}

	muxer := mp4.NewMuxer(f)
	if err = muxer.WriteHeader(codecs); err != nil {
		obj.stopWithError(session, err.Error())
		return
	}
	defer muxer.WriteTrailer()

	maxDuration := cfg.MaxDurationMinutes
	if maxDuration <= 0 {
		maxDuration = 60
	}
	stopTimer := time.NewTimer(time.Duration(maxDuration) * time.Minute)
	defer stopTimer.Stop()
	noVideo := time.NewTimer(20 * time.Second)
	defer noVideo.Stop()

	var videoStarted bool
	for {
		select {
		case <-session.stopCh:
			obj.finish(session)
			return
		case <-stopTimer.C:
			obj.finish(session)
			return
		case <-noVideo.C:
			obj.stopWithError(session, ErrorStreamNoVideo.Error())
			return
		case pck := <-ch:
			if pck == nil {
				continue
			}
			if pck.IsKeyFrame {
				videoStarted = true
				noVideo.Reset(20 * time.Second)
			}
			if !videoStarted {
				continue
			}
			if err = muxer.WritePacket(*pck); err != nil {
				obj.stopWithError(session, err.Error())
				return
			}
		}
	}
}

func (obj *RecordingManager) finish(session *RecordingSession) {
	obj.mutex.Lock()
	delete(obj.sessions, recordingKey(session.UUID, session.Channel))
	obj.mutex.Unlock()
}

func (obj *RecordingManager) stopWithError(session *RecordingSession, errMsg string) {
	session.Err = errMsg
	obj.finish(session)
}

func (obj *RecordingManager) Stop(uuid string, channel string) (*RecordingSession, error) {
	key := recordingKey(uuid, channel)
	obj.mutex.Lock()
	session, ok := obj.sessions[key]
	obj.mutex.Unlock()
	if !ok {
		return nil, ErrorRecordingNotFound
	}
	select {
	case <-session.stopCh:
	default:
		close(session.stopCh)
	}
	select {
	case <-session.doneCh:
	case <-time.After(3 * time.Second):
	}
	return session, nil
}

func (obj *RecordingManager) Status(uuid string, channel string) map[string]interface{} {
	obj.mutex.Lock()
	defer obj.mutex.Unlock()
	session, ok := obj.sessions[recordingKey(uuid, channel)]
	if !ok {
		return map[string]interface{}{
			"active": false,
		}
	}
	return map[string]interface{}{
		"active":     true,
		"id":         session.ID,
		"uuid":       session.UUID,
		"channel":    session.Channel,
		"file_path":  session.FilePath,
		"started_at": session.StartedAt.Unix(),
		"error":      session.Err,
	}
}
