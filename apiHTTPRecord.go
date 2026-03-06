package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func HTTPAPIRecordStart(c *gin.Context) {
	uuid := c.Param("uuid")
	channel := c.Param("channel")
	if !Storage.StreamChannelExist(uuid, channel) {
		c.IndentedJSON(http.StatusNotFound, Message{Status: 0, Payload: ErrorStreamNotFound.Error()})
		return
	}
	session, err := Recorder.Start(uuid, channel)
	if err != nil {
		status := http.StatusBadRequest
		if err == ErrorRecordingAlreadyRunning {
			status = http.StatusConflict
		}
		c.IndentedJSON(status, Message{Status: 0, Payload: err.Error()})
		return
	}
	c.IndentedJSON(http.StatusOK, Message{Status: 1, Payload: gin.H{
		"id":         session.ID,
		"file_path":  session.FilePath,
		"started_at": session.StartedAt.Unix(),
	}})
}

func HTTPAPIRecordStop(c *gin.Context) {
	uuid := c.Param("uuid")
	channel := c.Param("channel")
	session, err := Recorder.Stop(uuid, channel)
	if err != nil {
		c.IndentedJSON(http.StatusNotFound, Message{Status: 0, Payload: err.Error()})
		return
	}
	c.IndentedJSON(http.StatusOK, Message{Status: 1, Payload: gin.H{
		"id":        session.ID,
		"file_path": session.FilePath,
		"error":     session.Err,
	}})
}

func HTTPAPIRecordStatus(c *gin.Context) {
	c.IndentedJSON(http.StatusOK, Message{Status: 1, Payload: Recorder.Status(c.Param("uuid"), c.Param("channel"))})
}
