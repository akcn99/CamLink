package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"os"
	"time"

	"github.com/hashicorp/go-version"

	"github.com/imdario/mergo"

	"github.com/liip/sheriff"

	"github.com/sirupsen/logrus"
)

// Command line flag global variables
var debug bool
var configFile string

//NewStreamCore do load config file
func NewStreamCore() *StorageST {
	flag.BoolVar(&debug, "debug", true, "set debug mode")
	flag.StringVar(&configFile, "config", "config.json", "config patch (/etc/server/config.json or config.json)")
	flag.Parse()

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
	debug = tmp.Server.Debug
	for i, i2 := range tmp.Streams {
		for i3, i4 := range i2.Channels {
			channel := tmp.ChannelDefaults
			err = mergo.Merge(&channel, i4)
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
			i2.Channels[i3] = channel
		}
		tmp.Streams[i] = i2
	}
	return &tmp
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

//ClientDelete Delete Client
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
