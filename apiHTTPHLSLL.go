package main

import (
	"strings"

	"github.com/deepch/vdk/av"
	"github.com/deepch/vdk/format/mp4f"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// HTTPAPIServerStreamHLSLLInit send client ts segment
func HTTPAPIServerStreamHLSLLInit(c *gin.Context) {
	safeContext := c.Copy()
	requestLogger := log.WithFields(logrus.Fields{
		"module":  "http_hlsll",
		"stream":  safeContext.Param("uuid"),
		"channel": safeContext.Param("channel"),
		"func":    "HTTPAPIServerStreamHLSLLInit",
	})

	if !Storage.StreamChannelExist(safeContext.Param("uuid"), safeContext.Param("channel")) {
		c.IndentedJSON(500, Message{Status: 0, Payload: ErrorStreamNotFound.Error()})
		requestLogger.WithFields(logrus.Fields{
			"call": "StreamChannelExist",
		}).Errorln(ErrorStreamNotFound.Error())
		return
	}

	if !AuthorizeStreamAccess(c, "HLS", safeContext.Param("uuid"), safeContext.Param("channel")) {
		requestLogger.WithFields(logrus.Fields{
			"call": "AuthorizeStreamAccess",
		}).Errorln(ErrorStreamUnauthorized.Error())
		return
	}

	c.Header("Content-Type", "application/x-mpegURL")
	Storage.StreamChannelRun(safeContext.Param("uuid"), safeContext.Param("channel"))
	codecs, err := Storage.StreamChannelCodecs(safeContext.Param("uuid"), safeContext.Param("channel"))
	if err != nil {
		c.IndentedJSON(500, Message{Status: 0, Payload: err.Error()})
		requestLogger.WithFields(logrus.Fields{
			"call": "StreamChannelCodecs",
		}).Errorln(err.Error())
		return
	}
	codecs, _ = hlsllCodecsForRequest(c, codecs)
	Muxer := mp4f.NewMuxer(nil)
	err = Muxer.WriteHeader(codecs)
	if err != nil {
		c.IndentedJSON(500, Message{Status: 0, Payload: err.Error()})
		requestLogger.WithFields(logrus.Fields{
			"call": "WriteHeader",
		}).Errorln(err.Error())
		return
	}
	c.Header("Content-Type", "video/mp4")
	_, buf := Muxer.GetInit(codecs)
	_, err = c.Writer.Write(buf)
	if err != nil {
		c.IndentedJSON(500, Message{Status: 0, Payload: err.Error()})
		requestLogger.WithFields(logrus.Fields{
			"call": "Write",
		}).Errorln(err.Error())
		return
	}
}

// HTTPAPIServerStreamHLSLLM3U8 send client m3u8 play list
func HTTPAPIServerStreamHLSLLM3U8(c *gin.Context) {
	safeContext := c.Copy()
	requestLogger := log.WithFields(logrus.Fields{
		"module":  "http_hlsll",
		"stream":  safeContext.Param("uuid"),
		"channel": safeContext.Param("channel"),
		"func":    "HTTPAPIServerStreamHLSLLM3U8",
	})

	if !Storage.StreamChannelExist(safeContext.Param("uuid"), safeContext.Param("channel")) {
		c.IndentedJSON(500, Message{Status: 0, Payload: ErrorStreamNotFound.Error()})
		requestLogger.WithFields(logrus.Fields{
			"call": "StreamChannelExist",
		}).Errorln(ErrorStreamNotFound.Error())
		return
	}
	if !AuthorizeStreamAccess(c, "HLS", safeContext.Param("uuid"), safeContext.Param("channel")) {
		requestLogger.WithFields(logrus.Fields{
			"call": "AuthorizeStreamAccess",
		}).Errorln(ErrorStreamUnauthorized.Error())
		return
	}
	c.Header("Content-Type", "application/x-mpegURL")
	Storage.StreamChannelRun(safeContext.Param("uuid"), safeContext.Param("channel"))
	index, err := Storage.HLSMuxerM3U8(safeContext.Param("uuid"), safeContext.Param("channel"), stringToInt(safeContext.DefaultQuery("_HLS_msn", "-1")), stringToInt(safeContext.DefaultQuery("_HLS_part", "-1")))
	if err != nil {
		requestLogger.WithFields(logrus.Fields{
			"call": "HLSMuxerM3U8",
		}).Errorln(ErrorStreamNotFound.Error())
		return
	}
	if token := strings.TrimSpace(safeContext.Query("share_token")); token != "" {
		index = AppendQueryToM3U8(index, "share_token", token)
	}
	_, err = c.Writer.Write([]byte(index))
	if err != nil {
		requestLogger.WithFields(logrus.Fields{
			"call": "Write",
		}).Errorln(ErrorStreamNotFound.Error())
		return
	}
}

// HTTPAPIServerStreamHLSLLM4Segment send client ts segment
func HTTPAPIServerStreamHLSLLM4Segment(c *gin.Context) {
	requestLogger := log.WithFields(logrus.Fields{
		"module":  "http_hlsll",
		"stream":  c.Param("uuid"),
		"channel": c.Param("channel"),
		"func":    "HTTPAPIServerStreamHLSLLM4Segment",
	})

	c.Header("Content-Type", "video/mp4")
	if !Storage.StreamChannelExist(c.Param("uuid"), c.Param("channel")) {
		c.IndentedJSON(500, Message{Status: 0, Payload: ErrorStreamNotFound.Error()})
		requestLogger.WithFields(logrus.Fields{
			"call": "StreamChannelExist",
		}).Errorln(ErrorStreamNotFound.Error())
		return
	}
	if !AuthorizeStreamAccess(c, "HLS", c.Param("uuid"), c.Param("channel")) {
		requestLogger.WithFields(logrus.Fields{
			"call": "AuthorizeStreamAccess",
		}).Errorln(ErrorStreamUnauthorized.Error())
		return
	}
	codecs, err := Storage.StreamChannelCodecs(c.Param("uuid"), c.Param("channel"))
	if err != nil {
		c.IndentedJSON(500, Message{Status: 0, Payload: err.Error()})
		requestLogger.WithFields(logrus.Fields{
			"call": "StreamChannelCodecs",
		}).Errorln(err.Error())
		return
	}
	if codecs == nil {
		requestLogger.WithFields(logrus.Fields{
			"call": "StreamCodecs",
		}).Errorln("Codec Null")
		return
	}
	codecs, codecIndexMap := hlsllCodecsForRequest(c, codecs)
	Muxer := mp4f.NewMuxer(nil)
	err = Muxer.WriteHeader(codecs)
	if err != nil {
		requestLogger.WithFields(logrus.Fields{
			"call": "WriteHeader",
		}).Errorln(err.Error())
		return
	}
	seqData, err := Storage.HLSMuxerSegment(c.Param("uuid"), c.Param("channel"), stringToInt(c.Param("segment")))
	if err != nil {
		requestLogger.WithFields(logrus.Fields{
			"call": "HLSMuxerSegment",
		}).Errorln(err.Error())
		return
	}
	packets := hlsllPacketsForRequest(seqData, codecIndexMap)
	if len(packets) == 0 {
		requestLogger.WithFields(logrus.Fields{
			"call": "HLSMuxerSegment",
		}).Errorln("No matching packets after HLSLL codec filter")
		return
	}
	for _, packet := range packets {
		err = Muxer.WritePacket4(packet)
		if err != nil {
			requestLogger.WithFields(logrus.Fields{
				"call": "WritePacket4",
			}).Errorln(err.Error())
			return
		}
	}
	buf := Muxer.Finalize()
	_, err = c.Writer.Write(buf)
	if err != nil {
		requestLogger.WithFields(logrus.Fields{
			"call": "Write",
		}).Errorln(err.Error())
		return
	}
}

// HTTPAPIServerStreamHLSLLM4Fragment send client ts segment
func HTTPAPIServerStreamHLSLLM4Fragment(c *gin.Context) {
	requestLogger := log.WithFields(logrus.Fields{
		"module":  "http_hlsll",
		"stream":  c.Param("uuid"),
		"channel": c.Param("channel"),
		"func":    "HTTPAPIServerStreamHLSLLM4Fragment",
	})

	c.Header("Content-Type", "video/mp4")
	if !Storage.StreamChannelExist(c.Param("uuid"), c.Param("channel")) {
		c.IndentedJSON(500, Message{Status: 0, Payload: ErrorStreamNotFound.Error()})
		requestLogger.WithFields(logrus.Fields{
			"call": "StreamChannelExist",
		}).Errorln(ErrorStreamNotFound.Error())
		return
	}
	if !AuthorizeStreamAccess(c, "HLS", c.Param("uuid"), c.Param("channel")) {
		requestLogger.WithFields(logrus.Fields{
			"call": "AuthorizeStreamAccess",
		}).Errorln(ErrorStreamUnauthorized.Error())
		return
	}
	codecs, err := Storage.StreamChannelCodecs(c.Param("uuid"), c.Param("channel"))
	if err != nil {
		c.IndentedJSON(500, Message{Status: 0, Payload: err.Error()})
		requestLogger.WithFields(logrus.Fields{
			"call": "StreamChannelCodecs",
		}).Errorln(err.Error())
		return
	}
	if codecs == nil {
		requestLogger.WithFields(logrus.Fields{
			"call": "StreamCodecs",
		}).Errorln("Codec Null")
		return
	}
	codecs, codecIndexMap := hlsllCodecsForRequest(c, codecs)
	Muxer := mp4f.NewMuxer(nil)
	err = Muxer.WriteHeader(codecs)
	if err != nil {
		requestLogger.WithFields(logrus.Fields{
			"call": "WriteHeader",
		}).Errorln(err.Error())
		return
	}
	seqData, err := Storage.HLSMuxerFragment(c.Param("uuid"), c.Param("channel"), stringToInt(c.Param("segment")), stringToInt(c.Param("fragment")))
	if err != nil {
		requestLogger.WithFields(logrus.Fields{
			"call": "HLSMuxerFragment",
		}).Errorln(err.Error())
		return
	}
	packets := hlsllPacketsForRequest(seqData, codecIndexMap)
	if len(packets) == 0 {
		requestLogger.WithFields(logrus.Fields{
			"call": "HLSMuxerFragment",
		}).Errorln("No matching packets after HLSLL codec filter")
		return
	}
	for _, packet := range packets {
		err = Muxer.WritePacket4(packet)
		if err != nil {
			requestLogger.WithFields(logrus.Fields{
				"call": "WritePacket4",
			}).Errorln(err.Error())
			return
		}
	}
	buf := Muxer.Finalize()
	_, err = c.Writer.Write(buf)
	if err != nil {
		requestLogger.WithFields(logrus.Fields{
			"call": "Write",
		}).Errorln(err.Error())
		return
	}
}

func hlsllCodecsForRequest(c *gin.Context, codecs []av.CodecData) ([]av.CodecData, map[int]int) {
	if !hlsllNeedsVideoOnly(c, codecs) {
		return codecs, nil
	}
	filtered := make([]av.CodecData, 0, len(codecs))
	codecIndexMap := make(map[int]int, len(codecs))
	for idx, codec := range codecs {
		switch codec.Type() {
		case av.H264, av.H265:
			codecIndexMap[idx] = len(filtered)
			filtered = append(filtered, codec)
		}
	}
	if len(filtered) == 0 {
		return codecs, nil
	}
	return filtered, codecIndexMap
}

func hlsllNeedsVideoOnly(c *gin.Context, codecs []av.CodecData) bool {
	if len(codecs) < 2 {
		return false
	}
	var hasVideo bool
	var hasAAC bool
	for _, codec := range codecs {
		switch codec.Type() {
		case av.H264, av.H265:
			hasVideo = true
		case av.AAC:
			hasAAC = true
		}
	}
	if !hasVideo || !hasAAC {
		return false
	}
	userAgent := strings.ToLower(c.GetHeader("User-Agent"))
	if strings.Contains(userAgent, "iphone") || strings.Contains(userAgent, "ipad") || strings.Contains(userAgent, "ipod") {
		return true
	}
	return strings.Contains(userAgent, "macintosh") && strings.Contains(userAgent, "mobile")
}

func hlsllPacketsForRequest(seqData []*av.Packet, codecIndexMap map[int]int) []av.Packet {
	packets := make([]av.Packet, 0, len(seqData))
	for _, packet := range seqData {
		if packet == nil {
			continue
		}
		copyPacket := *packet
		if len(codecIndexMap) != 0 {
			newIdx, ok := codecIndexMap[int(copyPacket.Idx)]
			if !ok {
				continue
			}
			copyPacket.Idx = int8(newIdx)
		}
		packets = append(packets, copyPacket)
	}
	return packets
}
