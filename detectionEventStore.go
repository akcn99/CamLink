package main

import (
	"bytes"
	"database/sql"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type DetectionEventQuery struct {
	StreamUUID    string
	ChannelID     string
	ObjectClass   string
	StartedAfter  time.Time
	StartedBefore time.Time
}

type DetectionEventSummary struct {
	Total   int            `json:"total"`
	ByClass map[string]int `json:"by_class"`
}

type DetectionEventStoreStatus struct {
	Ready     bool   `json:"ready"`
	DBPath    string `json:"db_path"`
	ExportDir string `json:"export_dir"`
	Message   string `json:"message"`
}

type DetectionEventStore struct {
	mutex     sync.RWMutex
	db        *sql.DB
	dbPath    string
	exportDir string
	message   string
}

var DetectionEvents = &DetectionEventStore{}

func (store *DetectionEventStore) Init(dbPath string, exportDir string) error {
	dbPath = strings.TrimSpace(dbPath)
	exportDir = strings.TrimSpace(exportDir)
	if dbPath == "" {
		return fmt.Errorf("detection events db path is empty")
	}
	if err := ensureDir(filepath.Dir(dbPath)); err != nil {
		return err
	}
	if exportDir != "" {
		if err := ensureDir(exportDir); err != nil {
			return err
		}
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if _, err = db.Exec(`PRAGMA busy_timeout = 5000;`); err != nil {
		_ = db.Close()
		return err
	}
	if _, err = db.Exec(`PRAGMA journal_mode = WAL;`); err != nil {
		_ = db.Close()
		return err
	}
	if err = createDetectionEventSchema(db); err != nil {
		_ = db.Close()
		return err
	}

	store.mutex.Lock()
	oldDB := store.db
	store.db = db
	store.dbPath = dbPath
	store.exportDir = exportDir
	store.message = "ready"
	store.mutex.Unlock()

	if oldDB != nil {
		_ = oldDB.Close()
	}
	return nil
}

func (store *DetectionEventStore) Close() error {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	if store.db == nil {
		return nil
	}
	err := store.db.Close()
	store.db = nil
	store.message = "closed"
	return err
}

func (store *DetectionEventStore) Status() DetectionEventStoreStatus {
	store.mutex.RLock()
	defer store.mutex.RUnlock()
	status := DetectionEventStoreStatus{
		Ready:     store.db != nil,
		DBPath:    store.dbPath,
		ExportDir: store.exportDir,
		Message:   store.message,
	}
	if status.Message == "" {
		if status.Ready {
			status.Message = "ready"
		} else {
			status.Message = "not initialized"
		}
	}
	return status
}

func (store *DetectionEventStore) Append(event DetectionEventST) (DetectionEventST, error) {
	db, err := store.currentDB()
	if err != nil {
		return DetectionEventST{}, err
	}
	item := normalizeDetectionEvent(event)
	_, err = db.Exec(`
		INSERT INTO detection_events (
			event_id,
			stream_uuid,
			stream_name,
			channel_id,
			object_class,
			track_id,
			entered_at,
			entered_at_unix,
			snapshot_path,
			created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		item.EventID,
		item.StreamUUID,
		item.StreamName,
		item.ChannelID,
		item.ObjectClass,
		item.TrackID,
		item.EnteredAt.Format(time.RFC3339Nano),
		item.EnteredAt.Unix(),
		item.SnapshotPath,
		time.Now().UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return DetectionEventST{}, err
	}
	return item, nil
}

func (store *DetectionEventStore) List(query DetectionEventQuery) ([]DetectionEventST, DetectionEventSummary, error) {
	db, err := store.currentDB()
	if err != nil {
		return nil, DetectionEventSummary{}, err
	}
	where := make([]string, 0, 5)
	args := make([]interface{}, 0, 5)
	if value := strings.TrimSpace(query.StreamUUID); value != "" {
		where = append(where, "stream_uuid = ?")
		args = append(args, value)
	}
	if value := strings.TrimSpace(query.ChannelID); value != "" {
		where = append(where, "channel_id = ?")
		args = append(args, value)
	}
	if value := strings.ToLower(strings.TrimSpace(query.ObjectClass)); value != "" {
		where = append(where, "object_class = ?")
		args = append(args, value)
	}
	if !query.StartedAfter.IsZero() {
		where = append(where, "entered_at_unix >= ?")
		args = append(args, query.StartedAfter.UTC().Unix())
	}
	if !query.StartedBefore.IsZero() {
		where = append(where, "entered_at_unix <= ?")
		args = append(args, query.StartedBefore.UTC().Unix())
	}

	statement := `
		SELECT event_id, stream_uuid, stream_name, channel_id, object_class, track_id, entered_at, snapshot_path
		FROM detection_events
	`
	if len(where) != 0 {
		statement += " WHERE " + strings.Join(where, " AND ")
	}
	statement += " ORDER BY entered_at_unix DESC, event_id DESC"

	rows, err := db.Query(statement, args...)
	if err != nil {
		return nil, DetectionEventSummary{}, err
	}
	defer rows.Close()

	items := make([]DetectionEventST, 0)
	summary := DetectionEventSummary{ByClass: make(map[string]int)}
	for rows.Next() {
		var item DetectionEventST
		var enteredAt string
		if err := rows.Scan(
			&item.EventID,
			&item.StreamUUID,
			&item.StreamName,
			&item.ChannelID,
			&item.ObjectClass,
			&item.TrackID,
			&enteredAt,
			&item.SnapshotPath,
		); err != nil {
			return nil, DetectionEventSummary{}, err
		}
		item.EnteredAt = parseStoredDetectionTime(enteredAt)
		items = append(items, item)
		summary.Total++
		summary.ByClass[item.ObjectClass]++
	}
	if err := rows.Err(); err != nil {
		return nil, DetectionEventSummary{}, err
	}
	return items, summary, nil
}

func (store *DetectionEventStore) ExportCSV(query DetectionEventQuery) ([]byte, string, error) {
	items, _, err := store.List(query)
	if err != nil {
		return nil, "", err
	}
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	if err := writer.Write([]string{"event_id", "stream_uuid", "stream_name", "channel_id", "object_class", "track_id", "entered_at", "snapshot_path"}); err != nil {
		return nil, "", err
	}
	for _, event := range items {
		if err := writer.Write([]string{
			event.EventID,
			event.StreamUUID,
			event.StreamName,
			event.ChannelID,
			event.ObjectClass,
			event.TrackID,
			event.EnteredAt.Format(time.RFC3339),
			event.SnapshotPath,
		}); err != nil {
			return nil, "", err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, "", err
	}

	exportPath := ""
	status := store.Status()
	if status.ExportDir != "" {
		if err := ensureDir(status.ExportDir); err != nil {
			return nil, "", err
		}
		exportPath = filepath.Join(status.ExportDir, fmt.Sprintf("vehicle-entry-report-%s.csv", time.Now().Format("20060102-150405")))
		if err := os.WriteFile(exportPath, buf.Bytes(), 0644); err != nil {
			return nil, "", err
		}
	}
	return buf.Bytes(), exportPath, nil
}

func (store *DetectionEventStore) currentDB() (*sql.DB, error) {
	store.mutex.RLock()
	defer store.mutex.RUnlock()
	if store.db == nil {
		return nil, fmt.Errorf("detection event store is not initialized")
	}
	return store.db, nil
}

func createDetectionEventSchema(db *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS detection_events (
			event_id TEXT PRIMARY KEY,
			stream_uuid TEXT NOT NULL,
			stream_name TEXT NOT NULL,
			channel_id TEXT NOT NULL,
			object_class TEXT NOT NULL,
			track_id TEXT NOT NULL DEFAULT '',
			entered_at TEXT NOT NULL,
			entered_at_unix INTEGER NOT NULL,
			snapshot_path TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_detection_events_stream_channel_time ON detection_events(stream_uuid, channel_id, entered_at_unix DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_detection_events_class_time ON detection_events(object_class, entered_at_unix DESC);`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			return err
		}
	}
	return nil
}

func normalizeDetectionEvent(event DetectionEventST) DetectionEventST {
	if strings.TrimSpace(event.EventID) == "" {
		event.EventID = randomAlphaNum(18)
	}
	event.StreamUUID = strings.TrimSpace(event.StreamUUID)
	event.StreamName = strings.TrimSpace(event.StreamName)
	event.ChannelID = strings.TrimSpace(event.ChannelID)
	event.ObjectClass = strings.ToLower(strings.TrimSpace(event.ObjectClass))
	event.TrackID = strings.TrimSpace(event.TrackID)
	event.SnapshotPath = strings.TrimSpace(event.SnapshotPath)
	if event.EnteredAt.IsZero() {
		event.EnteredAt = time.Now()
	}
	event.EnteredAt = event.EnteredAt.UTC()
	return event
}

func parseStoredDetectionTime(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}
	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return parsed
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed
	}
	return time.Time{}
}

func ensureDir(dir string) error {
	dir = strings.TrimSpace(dir)
	if dir == "" || dir == "." {
		return nil
	}
	return os.MkdirAll(dir, 0755)
}
