// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package webrtcinternals provides types for marshaling/unmarshaling
// webrtc_internals_dump.txt downloaded from chrome://webrtc-internals.
package webrtcinternals

// Dump represents the entire contents of webrtc_internals_dump.txt.
type Dump struct {
	GetUserMedia    []GetUserMediaEntry                 `json:"getUserMedia,omitempty"`
	PeerConnections map[PeerConnectionID]PeerConnection `json:"PeerConnections,omitempty"`
	UserAgent       string                              `json:"UserAgent,omitempty"`
}

// GetUserMediaEntry represents an entry in the GetUserMedia field of Dump.
type GetUserMediaEntry struct {
	Rid            int     `json:"rid"`
	Pid            int     `json:"pid"`
	RequestID      int     `json:"request_id"`
	Origin         string  `json:"origin"`
	Timestamp      float64 `json:"timestamp"`
	StreamID       string  `json:"stream_id,omitempty"`
	Audio          string  `json:"audio,omitempty"`
	Video          string  `json:"video,omitempty"`
	AudioTrackInfo string  `json:"audio_track_info,omitempty"`
	VideoTrackInfo string  `json:"video_track_info,omitempty"`
}

// PeerConnection represents an entry in the PeerConnections field of Dump.
type PeerConnection struct {
	Pid              int      `json:"pid"`
	RTCConfiguration string   `json:"rtcConfiguration"`
	Constraints      string   `json:"constraints"`
	URL              string   `json:"url"`
	Stats            StatsMap `json:"stats"`
	UpdateLog        []Update `json:"updateLog"`
}

// Statistic represents an entry in the Stats field of PeerConnection.
type Statistic struct {
	StartTime TimeWithNanoseconds       `json:"startTime"`
	EndTime   TimeWithNanoseconds       `json:"endTime"`
	StatsType string                    `json:"statsType"`
	Values    SliceWithJSONQuotedString `json:"values"`
}

// Update represents an entry in the UpdateLog field of PeerConnection. The
// timestamp is unmarshaled based on the assumption that the dump was
// downloaded from chrome://webrtc-internals in the local time zone.
type Update struct {
	Time  TimeWithJSLocaleString `json:"time"`
	Type  string                 `json:"type"`
	Value string                 `json:"value"`
}
