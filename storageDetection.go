package main

import "time"

type DetectionOverviewItem struct {
	StreamUUID string             `json:"stream_uuid"`
	StreamName string             `json:"stream_name"`
	ChannelID  string             `json:"channel_id"`
	URL        string             `json:"url"`
	OnDemand   bool               `json:"on_demand"`
	Detection  DetectionChannelST `json:"detection"`
}

func (obj *StorageST) DetectionOverview() []DetectionOverviewItem {
	obj.mutex.RLock()
	defer obj.mutex.RUnlock()

	items := make([]DetectionOverviewItem, 0)
	for streamUUID, stream := range obj.Streams {
		for channelID, channel := range stream.Channels {
			items = append(items, DetectionOverviewItem{
				StreamUUID: streamUUID,
				StreamName: stream.Name,
				ChannelID:  channelID,
				URL:        channel.URL,
				OnDemand:   channel.OnDemand,
				Detection:  channel.Detection,
			})
		}
	}
	return items
}

func (obj *StorageST) DetectionChannelConfig(uuid string, channelID string) (DetectionChannelST, error) {
	obj.mutex.RLock()
	defer obj.mutex.RUnlock()

	stream, ok := obj.Streams[uuid]
	if !ok {
		return DetectionChannelST{}, ErrorStreamNotFound
	}
	channel, ok := stream.Channels[channelID]
	if !ok {
		return DetectionChannelST{}, ErrorStreamChannelNotFound
	}
	return channel.Detection, nil
}

func (obj *StorageST) DetectionChannelUpdate(uuid string, channelID string, detection DetectionChannelST) error {
	obj.mutex.Lock()
	defer obj.mutex.Unlock()

	stream, ok := obj.Streams[uuid]
	if !ok {
		return ErrorStreamNotFound
	}
	channel, ok := stream.Channels[channelID]
	if !ok {
		return ErrorStreamChannelNotFound
	}
	channel.Detection = detection
	applyChannelDefaults(&channel)
	stream.Channels[channelID] = channel
	obj.Streams[uuid] = stream
	return obj.SaveConfig()
}

func (obj *StorageST) DetectionSettingsUpdate(detection DetectionServerST) error {
	obj.mutex.Lock()
	defer obj.mutex.Unlock()

	obj.Server.Detection = detection
	applyServerDefaults(&obj.Server)
	return obj.SaveConfig()
}

func (obj *StorageST) TouchDetectionChannel(uuid string, channelID string) {
	obj.mutex.Lock()
	defer obj.mutex.Unlock()

	stream, ok := obj.Streams[uuid]
	if !ok {
		return
	}
	channel, ok := stream.Channels[channelID]
	if !ok {
		return
	}
	channel.ack = time.Now()
	stream.Channels[channelID] = channel
	obj.Streams[uuid] = stream
}
