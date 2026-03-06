package main

import (
	"errors"
	"net"
	"sync"
	"time"

	"github.com/deepch/vdk/av"
	"github.com/sirupsen/logrus"
)

var Storage = NewStreamCore()

// Default stream  type
const (
	MSE = iota
	WEBRTC
	RTSP
)

// Default stream status type
const (
	OFFLINE = iota
	ONLINE
)

// Default stream errors
var (
	Success                         = "success"
	ErrorStreamNotFound             = errors.New("stream not found")
	ErrorStreamAlreadyExists        = errors.New("stream already exists")
	ErrorStreamChannelAlreadyExists = errors.New("stream channel already exists")
	ErrorStreamNotHLSSegments       = errors.New("stream hls not ts seq found")
	ErrorStreamNoVideo              = errors.New("stream no video")
	ErrorStreamNoClients            = errors.New("stream no clients")
	ErrorStreamRestart              = errors.New("stream restart")
	ErrorStreamStopCoreSignal       = errors.New("stream stop core signal")
	ErrorStreamStopRTSPSignal       = errors.New("stream stop rtsp signal")
	ErrorStreamChannelNotFound      = errors.New("stream channel not found")
	ErrorStreamChannelCodecNotFound = errors.New("stream channel codec not ready, possible stream offline")
	ErrorStreamsLen0                = errors.New("streams len zero")
	ErrorStreamUnauthorized         = errors.New("stream request unauthorized")
	ErrorUnauthorized               = errors.New("unauthorized")
	ErrorInvalidCredentials         = errors.New("invalid credentials")
	ErrorShareNotFound              = errors.New("share not found")
	ErrorShareExpired               = errors.New("share expired")
	ErrorShareRevoked               = errors.New("share revoked")
	ErrorSharePasswordInvalid       = errors.New("share password invalid")
	ErrorShareConnectionLimit       = errors.New("share connection limit reached")
	ErrorShareTokenInvalid          = errors.New("share token invalid")
	ErrorRecordingAlreadyRunning    = errors.New("recording already running")
	ErrorRecordingNotFound          = errors.New("recording not found")
)

// StorageST main storage struct
type StorageST struct {
	mutex           sync.RWMutex
	Server          ServerST            `json:"server" groups:"api,config"`
	Streams         map[string]StreamST `json:"streams,omitempty" groups:"api,config"`
	ChannelDefaults ChannelST           `json:"channel_defaults,omitempty" groups:"api,config"`
}

// ServerST server storage section
type ServerST struct {
	Debug              bool         `json:"debug" groups:"api,config"`
	LogLevel           logrus.Level `json:"log_level" groups:"api,config"`
	HTTPDemo           bool         `json:"http_demo" groups:"api,config"`
	HTTPDebug          bool         `json:"http_debug" groups:"api,config"`
	HTTPLogin          string       `json:"http_login" groups:"api,config"`
	HTTPPassword       string       `json:"http_password" groups:"api,config"`
	HTTPAuth           bool         `json:"http_auth" groups:"api,config"`
	HTTPDir            string       `json:"http_dir" groups:"api,config"`
	HTTPPort           string       `json:"http_port" groups:"api,config"`
	RTSPPort           string       `json:"rtsp_port" groups:"api,config"`
	HTTPS              bool         `json:"https" groups:"api,config"`
	HTTPSPort          string       `json:"https_port" groups:"api,config"`
	HTTPSCert          string       `json:"https_cert" groups:"api,config"`
	HTTPSKey           string       `json:"https_key" groups:"api,config"`
	HTTPSAutoTLSEnable bool         `json:"https_auto_tls" groups:"api,config"`
	HTTPSAutoTLSName   string       `json:"https_auto_tls_name" groups:"api,config"`
	ICEServers         []string     `json:"ice_servers" groups:"api,config"`
	ICEUsername        string       `json:"ice_username" groups:"api,config"`
	ICECredential      string       `json:"ice_credential" groups:"api,config"`
	ICECandidates      []string     `json:"ice_candidates" groups:"api,config"`
	Token              Token        `json:"token,omitempty" groups:"api,config"`
	WebRTCPortMin      uint16       `json:"webrtc_port_min" groups:"api,config"`
	WebRTCPortMax      uint16       `json:"webrtc_port_max" groups:"api,config"`
	UILanguageDefault  string       `json:"ui_language_default,omitempty" groups:"api,config"`
	Recording          RecordingST  `json:"recording,omitempty" groups:"api,config"`
	Share              ShareST      `json:"share,omitempty" groups:"api,config"`
	AdminPasswordHash  string       `json:"admin_password_hash,omitempty" groups:"api,config"`
	SessionTTLMinutes  int          `json:"session_ttl_minutes,omitempty" groups:"api,config"`
}

// Token auth
type Token struct {
	Enable  bool   `json:"enable" groups:"api,config"`
	Backend string `json:"backend" groups:"api,config"`
}

type RecordingST struct {
	SavePath           string `json:"save_path,omitempty" groups:"api,config"`
	Resolution         string `json:"resolution,omitempty" groups:"api,config"`
	VideoCodec         string `json:"video_codec,omitempty" groups:"api,config"`
	Format             string `json:"format,omitempty" groups:"api,config"`
	FilenameRule       string `json:"filename_rule,omitempty" groups:"api,config"`
	MaxDurationMinutes int    `json:"max_duration_minutes,omitempty" groups:"api,config"`
}

type ShareST struct {
	SignSecret           string `json:"sign_secret,omitempty" groups:"api,config"`
	DefaultExpireMinutes int    `json:"default_expire_minutes,omitempty" groups:"api,config"`
	DefaultMaxConnections int   `json:"default_max_connections,omitempty" groups:"api,config"`
}

// ServerST stream storage section
type StreamST struct {
	Name     string               `json:"name,omitempty" groups:"api,config"`
	Channels map[string]ChannelST `json:"channels,omitempty" groups:"api,config"`
}

type ChannelST struct {
	Name               string `json:"name,omitempty" groups:"api,config"`
	URL                string `json:"url,omitempty" groups:"api,config"`
	OnDemand           bool   `json:"on_demand,omitempty" groups:"api,config"`
	Debug              bool   `json:"debug,omitempty" groups:"api,config"`
	Status             int    `json:"status,omitempty" groups:"api"`
	InsecureSkipVerify bool   `json:"insecure_skip_verify,omitempty" groups:"api,config"`
	Audio              bool   `json:"audio,omitempty" groups:"api,config"`
	runLock            bool
	codecs             []av.CodecData
	sdp                []byte
	signals            chan int
	hlsSegmentBuffer   map[int]SegmentOld
	hlsSegmentNumber   int
	hlsSequence        int
	hlsLastDur         int
	clients            map[string]ClientST
	ack                time.Time
	hlsMuxer           *MuxerHLS `json:"-"`
}

// ClientST client storage section
type ClientST struct {
	mode              int
	signals           chan int
	outgoingAVPacket  chan *av.Packet
	outgoingRTPPacket chan *[]byte
	socket            net.Conn
}

// SegmentOld HLS cache section
type SegmentOld struct {
	dur  time.Duration
	data []*av.Packet
}
