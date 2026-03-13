package main

import (
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/autotls"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// Message resp struct
type Message struct {
	Status  int         `json:"status"`
	Payload interface{} `json:"payload"`
}

// HTTPAPIServer start http server routes
func HTTPAPIServer() {
	//Set HTTP API mode
	log.WithFields(logrus.Fields{
		"module": "http_server",
		"func":   "RTSPServer",
		"call":   "Start",
	}).Infoln("Server HTTP start")
	var public *gin.Engine
	if !Storage.ServerHTTPDebug() {
		gin.SetMode(gin.ReleaseMode)
		public = gin.New()
	} else {
		gin.SetMode(gin.DebugMode)
		public = gin.Default()
	}

	public.Use(CrossOrigin())
	public.Use(func(c *gin.Context) {
		c.Next()
		if len(c.Errors) == 0 {
			return
		}
		for _, err := range c.Errors {
			log.WithFields(logrus.Fields{
				"module": "http_server",
				"func":   "ErrorMiddleware",
				"path":   c.Request.URL.Path,
				"method": c.Request.Method,
			}).Errorln(err.Error())
		}
	})
	admin := public.Group("/")
	admin.Use(AdminAuthMiddleware())

	/*
		Static HTML Files Demo Mode
	*/

	if Storage.ServerHTTPDemo() {
		public.LoadHTMLGlob(Storage.ServerHTTPDir() + "/templates/*")
		public.GET("/login", HTTPAPILoginPage)
		public.POST("/api/auth/login", HTTPAPIAuthLogin)
		public.POST("/api/auth/logout", HTTPAPIAuthLogout)
		public.GET("/share/watch/:id", HTTPAPIShareWatchPage)
		public.POST("/share/:id/session/start", HTTPAPIShareSessionStart)
		public.POST("/share/:id/session/heartbeat", HTTPAPIShareSessionHeartbeat)
		public.POST("/share/:id/session/stop", HTTPAPIShareSessionStop)

		admin.GET("/", HTTPAPIServerIndex)
		admin.GET("/pages/stream/list", HTTPAPIStreamList)
		admin.GET("/pages/stream/add", HTTPAPIAddStream)
		admin.GET("/pages/stream/edit/:uuid", HTTPAPIEditStream)
		admin.GET("/pages/player/hls/:uuid/:channel", HTTPAPIPlayHls)
		admin.GET("/pages/player/mse/:uuid/:channel", HTTPAPIPlayMse)
		admin.GET("/pages/player/webrtc/:uuid/:channel", HTTPAPIPlayWebrtc)
		admin.GET("/pages/multiview", HTTPAPIMultiview)
		admin.Any("/pages/multiview/full", HTTPAPIFullScreenMultiView)
		admin.GET("/pages/documentation", HTTPAPIServerDocumentation)
		admin.GET("/pages/player/all/:uuid/:channel", HTTPAPIPlayAll)
		admin.GET("/pages/settings", HTTPAPISettingsPage)
		admin.GET("/pages/detection", HTTPAPIDetectionPage)
		public.StaticFS("/static", http.Dir(Storage.ServerHTTPDir()+"/static"))
	}

	public.GET("/detector/config", HTTPAPIDetectorConfig)
	public.POST("/detector/events", HTTPAPIDetectorEventIngest)

	/*
		Stream Control elements
	*/

	admin.GET("/streams", HTTPAPIServerStreams)
	admin.POST("/stream/:uuid/add", HTTPAPIServerStreamAdd)
	admin.POST("/stream/:uuid/edit", HTTPAPIServerStreamEdit)
	admin.GET("/stream/:uuid/delete", HTTPAPIServerStreamDelete)
	admin.GET("/stream/:uuid/reload", HTTPAPIServerStreamReload)
	admin.GET("/stream/:uuid/info", HTTPAPIServerStreamInfo)

	/*
		Streams Multi Control elements
	*/

	admin.POST("/streams/multi/control/add", HTTPAPIServerStreamsMultiControlAdd)
	admin.POST("/streams/multi/control/delete", HTTPAPIServerStreamsMultiControlDelete)

	/*
		Stream Channel elements
	*/

	admin.POST("/stream/:uuid/channel/:channel/add", HTTPAPIServerStreamChannelAdd)
	admin.POST("/stream/:uuid/channel/:channel/edit", HTTPAPIServerStreamChannelEdit)
	admin.GET("/stream/:uuid/channel/:channel/delete", HTTPAPIServerStreamChannelDelete)
	admin.GET("/stream/:uuid/channel/:channel/codec", HTTPAPIServerStreamChannelCodec)
	admin.GET("/stream/:uuid/channel/:channel/reload", HTTPAPIServerStreamChannelReload)
	admin.GET("/stream/:uuid/channel/:channel/info", HTTPAPIServerStreamChannelInfo)
	admin.GET("/stream/:uuid/channel/:channel/record/status", HTTPAPIRecordStatus)
	admin.POST("/stream/:uuid/channel/:channel/record/start", HTTPAPIRecordStart)
	admin.POST("/stream/:uuid/channel/:channel/record/stop", HTTPAPIRecordStop)

	admin.GET("/settings/global", HTTPAPIGetGlobalSettings)
	admin.POST("/settings/global", HTTPAPIUpdateGlobalSettings)
	admin.POST("/settings/admin/password", HTTPAPIUpdateAdminPassword)
	admin.GET("/detection/settings/global", HTTPAPIGetDetectionSettings)
	admin.POST("/detection/settings/global", HTTPAPIUpdateDetectionSettings)
	admin.GET("/detection/overview", HTTPAPIGetDetectionOverview)
	admin.GET("/detection/service/status", HTTPAPIGetDetectionServiceStatus)
	admin.GET("/detection/snapshot/:uuid/:channel", HTTPAPIGetDetectionSnapshot)
	admin.GET("/detection/config/:uuid/:channel", HTTPAPIGetDetectionConfig)
	admin.POST("/detection/config/:uuid/:channel", HTTPAPIUpdateDetectionConfig)
	admin.GET("/detection/events", HTTPAPIGetDetectionEvents)
	admin.GET("/detection/events/export", HTTPAPIExportDetectionEvents)
	admin.POST("/detection/events/ingest", HTTPAPIIngestDetectionEvent)
	admin.POST("/detection/events/mock/:uuid/:channel", HTTPAPICreateDetectionMockEvent)
	admin.POST("/share/create", HTTPAPIShareCreate)
	admin.GET("/share/:id/status", HTTPAPIShareStatus)
	admin.POST("/share/:id/revoke", HTTPAPIShareRevoke)

	/*
		Stream video elements
	*/
	//HLS
	public.GET("/stream/:uuid/channel/:channel/hls/live/index.m3u8", HTTPAPIServerStreamHLSM3U8)
	public.GET("/stream/:uuid/channel/:channel/hls/live/segment/:seq/file.ts", HTTPAPIServerStreamHLSTS)
	//HLS remote record
	//public.GET("/stream/:uuid/channel/:channel/hls/rr/:s/:e/index.m3u8", HTTPAPIServerStreamRRM3U8)
	//public.GET("/stream/:uuid/channel/:channel/hls/rr/:s/:e/:seq/file.ts", HTTPAPIServerStreamRRTS)
	//HLS LL
	public.GET("/stream/:uuid/channel/:channel/hlsll/live/index.m3u8", HTTPAPIServerStreamHLSLLM3U8)
	public.GET("/stream/:uuid/channel/:channel/hlsll/live/init.mp4", HTTPAPIServerStreamHLSLLInit)
	public.GET("/stream/:uuid/channel/:channel/hlsll/live/segment/:segment/:any", HTTPAPIServerStreamHLSLLM4Segment)
	public.GET("/stream/:uuid/channel/:channel/hlsll/live/fragment/:segment/:fragment/:any", HTTPAPIServerStreamHLSLLM4Fragment)
	//MSE
	public.GET("/stream/:uuid/channel/:channel/mse", HTTPAPIServerStreamMSE)
	public.POST("/stream/:uuid/channel/:channel/webrtc", HTTPAPIServerStreamWebRTC)
	//Save fragment to mp4
	public.GET("/stream/:uuid/channel/:channel/save/mp4/fragment/:duration", HTTPAPIServerStreamSaveToMP4)
	/*
		HTTPS Mode Cert
		# Key considerations for algorithm "RSA" ≥ 2048-bit
		openssl genrsa -out server.key 2048

		# Key considerations for algorithm "ECDSA" ≥ secp384r1
		# List ECDSA the supported curves (openssl ecparam -list_curves)
		#openssl ecparam -genkey -name secp384r1 -out server.key
		#Generation of self-signed(x509) public key (PEM-encodings .pem|.crt) based on the private (.key)

		openssl req -new -x509 -sha256 -key server.key -out server.crt -days 3650
	*/
	if Storage.ServerHTTPS() {
		if Storage.ServerHTTPSAutoTLSEnable() {
			go func() {
				err := autotls.Run(public, Storage.ServerHTTPSAutoTLSName()+Storage.ServerHTTPSPort())
				if err != nil {
					log.Println("Start HTTPS Server Error", err)
				}
			}()
		} else {
			go func() {
				err := public.RunTLS(Storage.ServerHTTPSPort(), Storage.ServerHTTPSCert(), Storage.ServerHTTPSKey())
				if err != nil {
					log.WithFields(logrus.Fields{
						"module": "http_router",
						"func":   "HTTPSAPIServer",
						"call":   "ServerHTTPSPort",
					}).Fatalln(err.Error())
					os.Exit(1)
				}
			}()
		}
	}
	err := public.Run(Storage.ServerHTTPPort())
	if err != nil {
		log.WithFields(logrus.Fields{
			"module": "http_router",
			"func":   "HTTPAPIServer",
			"call":   "ServerHTTPPort",
		}).Fatalln(err.Error())
		os.Exit(1)
	}

}

// HTTPAPIServerIndex index file
func HTTPAPIServerIndex(c *gin.Context) {
	c.HTML(http.StatusOK, "index.tmpl", gin.H{
		"port":    Storage.ServerHTTPPort(),
		"streams": Storage.Streams,
		"version": time.Now().String(),
		"page":    "index",
		"ui_lang": Storage.ServerUILanguageDefault(),
	})

}

func HTTPAPIServerDocumentation(c *gin.Context) {
	c.HTML(http.StatusOK, "documentation.tmpl", gin.H{
		"port":    Storage.ServerHTTPPort(),
		"streams": Storage.Streams,
		"version": time.Now().String(),
		"page":    "documentation",
		"ui_lang": Storage.ServerUILanguageDefault(),
	})
}

func HTTPAPIStreamList(c *gin.Context) {
	c.HTML(http.StatusOK, "stream_list.tmpl", gin.H{
		"port":    Storage.ServerHTTPPort(),
		"streams": Storage.Streams,
		"version": time.Now().String(),
		"page":    "stream_list",
		"ui_lang": Storage.ServerUILanguageDefault(),
	})
}

func HTTPAPIPlayHls(c *gin.Context) {
	c.HTML(http.StatusOK, "play_hls.tmpl", gin.H{
		"port":    Storage.ServerHTTPPort(),
		"streams": Storage.Streams,
		"version": time.Now().String(),
		"page":    "play_hls",
		"uuid":    c.Param("uuid"),
		"channel": c.Param("channel"),
		"ui_lang": Storage.ServerUILanguageDefault(),
	})
}
func HTTPAPIPlayMse(c *gin.Context) {
	c.HTML(http.StatusOK, "play_mse.tmpl", gin.H{
		"port":    Storage.ServerHTTPPort(),
		"streams": Storage.Streams,
		"version": time.Now().String(),
		"page":    "play_mse",
		"uuid":    c.Param("uuid"),
		"channel": c.Param("channel"),
		"ui_lang": Storage.ServerUILanguageDefault(),
	})
}
func HTTPAPIPlayWebrtc(c *gin.Context) {
	c.HTML(http.StatusOK, "play_webrtc.tmpl", gin.H{
		"port":    Storage.ServerHTTPPort(),
		"streams": Storage.Streams,
		"version": time.Now().String(),
		"page":    "play_webrtc",
		"uuid":    c.Param("uuid"),
		"channel": c.Param("channel"),
		"ui_lang": Storage.ServerUILanguageDefault(),
	})
}
func HTTPAPIAddStream(c *gin.Context) {
	c.HTML(http.StatusOK, "add_stream.tmpl", gin.H{
		"port":    Storage.ServerHTTPPort(),
		"streams": Storage.Streams,
		"version": time.Now().String(),
		"page":    "add_stream",
		"ui_lang": Storage.ServerUILanguageDefault(),
	})
}
func HTTPAPIEditStream(c *gin.Context) {
	c.HTML(http.StatusOK, "edit_stream.tmpl", gin.H{
		"port":    Storage.ServerHTTPPort(),
		"streams": Storage.Streams,
		"version": time.Now().String(),
		"page":    "edit_stream",
		"uuid":    c.Param("uuid"),
		"ui_lang": Storage.ServerUILanguageDefault(),
	})
}

func HTTPAPIMultiview(c *gin.Context) {
	c.HTML(http.StatusOK, "multiview.tmpl", gin.H{
		"port":    Storage.ServerHTTPPort(),
		"streams": Storage.Streams,
		"version": time.Now().String(),
		"page":    "multiview",
		"ui_lang": Storage.ServerUILanguageDefault(),
	})
}

func HTTPAPIPlayAll(c *gin.Context) {
	c.HTML(http.StatusOK, "play_all.tmpl", gin.H{
		"port":    Storage.ServerHTTPPort(),
		"streams": Storage.Streams,
		"version": time.Now().String(),
		"page":    "play_all",
		"uuid":    c.Param("uuid"),
		"channel": c.Param("channel"),
		"ui_lang": Storage.ServerUILanguageDefault(),
	})
}

type MultiViewOptions struct {
	Grid   int                             `json:"grid"`
	Player map[string]MultiViewOptionsGrid `json:"player"`
}
type MultiViewOptionsGrid struct {
	UUID       string `json:"uuid"`
	Channel    int    `json:"channel"`
	PlayerType string `json:"playerType"`
}

func HTTPAPIFullScreenMultiView(c *gin.Context) {
	var createParams MultiViewOptions
	err := c.ShouldBindJSON(&createParams)
	if err != nil {
		log.WithFields(logrus.Fields{
			"module": "http_page",
			"func":   "HTTPAPIFullScreenMultiView",
			"call":   "BindJSON",
		}).Errorln(err.Error())
	}
	log.WithFields(logrus.Fields{
		"module": "http_page",
		"func":   "HTTPAPIFullScreenMultiView",
		"call":   "Options",
	}).Debugln(createParams)
	c.HTML(http.StatusOK, "fullscreenmulti.tmpl", gin.H{
		"port":    Storage.ServerHTTPPort(),
		"streams": Storage.Streams,
		"version": time.Now().String(),
		"options": createParams,
		"page":    "fullscreenmulti",
		"query":   c.Request.URL.Query(),
		"ui_lang": Storage.ServerUILanguageDefault(),
	})
}

// CrossOrigin Access-Control-Allow-Origin any methods
func CrossOrigin() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
