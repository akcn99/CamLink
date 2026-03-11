package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/hashicorp/go-version"
	"github.com/imdario/mergo"
	"github.com/liip/sheriff"
	"github.com/sirupsen/logrus"
)

// Command line flag global variables
var debug bool
var configFile string

// NewStreamCore do load config file
func NewStreamCore() *StorageST {
	flag.BoolVar(&debug, "debug", true, "set debug mode")
	flag.StringVar(&configFile, "config", "", "config path (default: config.local.json, fallback to config.json or config.example.json)")
	flag.Parse()
	configFile = resolveConfigPath(configFile)

	var tmp StorageST
	data, err := ioutil.ReadFile(configFile)
	if err != nil {
		log.WithFields(logrus.Fields{
			"module": "config",
			"func":   "NewStreamCore",
			"call":   "ReadFile",
		}).Errorln(err.Error())
		os.Exit(1)
	}
	err = json.Unmarshal(data, &tmp)
	if err != nil {
		log.WithFields(logrus.Fields{
			"module": "config",
			"func":   "NewStreamCore",
			"call":   "Unmarshal",
		}).Errorln(err.Error())
		os.Exit(1)
	}
	applyServerDefaults(&tmp.Server)
	applyChannelDefaults(&tmp.ChannelDefaults)
	debug = tmp.Server.Debug
	for i, i2 := range tmp.Streams {
		for i3, i4 := range i2.Channels {
			channel := tmp.ChannelDefaults
			err = mergo.Merge(&channel, i4, mergo.WithOverride)
			if err != nil {
				log.WithFields(logrus.Fields{
					"module": "config",
					"func":   "NewStreamCore",
					"call":   "Merge",
				}).Errorln(err.Error())
				os.Exit(1)
			}
			channel.clients = make(map[string]ClientST)
			channel.ack = time.Now().Add(-255 * time.Hour)
			channel.hlsSegmentBuffer = make(map[int]SegmentOld)
			channel.signals = make(chan int, 100)
			applyChannelDefaults(&channel)
			i2.Channels[i3] = channel
		}
		tmp.Streams[i] = i2
	}
	return &tmp
}

func resolveConfigPath(path string) string {
	path = strings.TrimSpace(path)
	if path != "" {
		return path
	}
	candidates := []string{"config.local.json", "config.json", "config.example.json"}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return "config.local.json"
}

func applyServerDefaults(server *ServerST) {
	if server.HTTPDir == "" {
		server.HTTPDir = DefaultHTTPDir
	}
	if server.HTTPPort == "" {
		server.HTTPPort = ":8083"
	}
	if server.RTSPPort == "" {
		server.RTSPPort = ":5541"
	}
	if server.UILanguageDefault == "" {
		server.UILanguageDefault = "zh-CN"
	}
	if server.Recording.SavePath == "" {
		server.Recording.SavePath = "save"
	}
	if server.Recording.Format == "" {
		server.Recording.Format = "mp4"
	}
	if server.Recording.FilenameRule == "" {
		server.Recording.FilenameRule = "stream_channel_timestamp"
	}
	if server.Recording.Resolution == "" {
		server.Recording.Resolution = "source"
	}
	if server.Recording.VideoCodec == "" {
		server.Recording.VideoCodec = "source"
	}
	if server.Recording.MaxDurationMinutes <= 0 {
		server.Recording.MaxDurationMinutes = 60
	}
	if server.Share.DefaultExpireMinutes <= 0 {
		server.Share.DefaultExpireMinutes = 60
	}
	if server.Share.DefaultMaxConnections < 1 || server.Share.DefaultMaxConnections > 5 {
		server.Share.DefaultMaxConnections = 1
	}
	if server.Share.SignSecret == "" {
		server.Share.SignSecret = randomAlphaNum(32)
	}
	if server.Detection.DetectorURL == "" {
		server.Detection.DetectorURL = "http://camlink-detector:8091"
	}
	if server.Detection.EventsDBPath == "" {
		server.Detection.EventsDBPath = "save/detection-events.db"
	}
	if server.Detection.ExportDir == "" {
		server.Detection.ExportDir = "save/reports"
	}
	if server.Detection.AccessToken == "" {
		server.Detection.AccessToken = randomAlphaNum(32)
	}
	if server.HTTPLogin == "" {
		server.HTTPLogin = "admin"
	}
	if server.AdminPasswordHash == "" {
		source := server.HTTPPassword
		if source == "" {
			source = "admin1234"
		}
		if hash, err := HashPassword(source); err == nil {
			server.AdminPasswordHash = hash
		}
	}
	if server.SessionTTLMinutes <= 0 {
		server.SessionTTLMinutes = 720
	}
}

func applyChannelDefaults(channel *ChannelST) {
	// Most cameras are expected to have audio; enable it by default unless
	// explicitly configured in the future by a dedicated field.
	if channel.Audio == nil {
		enabled := true
		channel.Audio = &enabled
	}
	if channel.Detection.Mode == "" {
		channel.Detection.Mode = "vehicle_entry"
	}
	if channel.Detection.SampleFPS <= 0 {
		channel.Detection.SampleFPS = 1
	}
	if channel.Detection.CooldownSeconds <= 0 {
		channel.Detection.CooldownSeconds = 30
	}
	if channel.Detection.ConfidenceThreshold <= 0 {
		channel.Detection.ConfidenceThreshold = 0.35
	}
	if channel.Detection.MinBoxArea < 0 {
		channel.Detection.MinBoxArea = 0
	}
	if channel.Detection.MinMovePixels < 0 {
		channel.Detection.MinMovePixels = 0
	}
	if strings.TrimSpace(channel.Detection.EntryDirection) == "" {
		channel.Detection.EntryDirection = "any"
	}
	if len(channel.Detection.Classes) == 0 {
		channel.Detection.Classes = []string{"car", "motorcycle", "bicycle"}
	}
	if channel.Detection.TriggerConsecutiveFrames <= 0 {
		channel.Detection.TriggerConsecutiveFrames = 2
	}
}

// ClientDelete Delete Client
func (obj *StorageST) SaveConfig() error {
	log.WithFields(logrus.Fields{
		"module": "config",
		"func":   "NewStreamCore",
	}).Debugln("Saving configuration to", configFile)
	v2, err := version.NewVersion("2.0.0")
	if err != nil {
		return err
	}
	data, err := sheriff.Marshal(&sheriff.Options{
		Groups:     []string{"config"},
		ApiVersion: v2,
	}, obj)
	if err != nil {
		return err
	}
	res, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(configFile, res, 0644)
	if err != nil {
		log.WithFields(logrus.Fields{
			"module": "config",
			"func":   "SaveConfig",
			"call":   "WriteFile",
		}).Errorln(err.Error())
		return err
	}
	return nil
}
