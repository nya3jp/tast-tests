// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package webcodecs provides common code for video.WebCodecs* tests
package webcodecs

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/videotype"
	"chromiumos/tast/testing"
)

// HardwareAcceleration represents the preference of used codecs in WebCodecs API.
// See https://www.w3.org/TR/webcodecs/#hardware-acceleration.
type HardwareAcceleration string

const (
	// PreferHardware means hardware accelerated encoder/decoder is preferred.
	PreferHardware HardwareAcceleration = "prefer-hardware"
	// PreferSoftware means software encoder/decoder is preferred.
	PreferSoftware HardwareAcceleration = "prefer-software"
)

type videoConfig struct {
	width, height, numFrames, framerate int
}

// MP4DemuxerDataFiles returns the list of JS files for demuxing MP4 container.
func MP4DemuxerDataFiles() []string {
	return []string{
		"third_party/mp4/mp4_demuxer.js",
		"third_party/mp4/mp4box.all.min.js",
	}
}

// toMIMECodec converts videotype.Codec to codec in MIME type.
// See https://developer.mozilla.org/en-US/docs/Web/Media/Formats/codecs_parameter for detail.
func toMIMECodec(codec videotype.Codec) string {
	switch codec {
	case videotype.H264:
		// H.264 Baseline Level 3.1.
		return "avc1.42001E"
	case videotype.VP8:
		return "vp8"
	case videotype.VP9:
		// VP9 profile 0 level 1.0 8-bit depth.
		return "vp09.00.10.08"
	}
	return ""
}

func outputJSLogAndError(ctx context.Context, conn *chrome.Conn, callErr error) error {
	var logs string
	if err := conn.Eval(ctx, "TEST.getLogs()", &logs); err != nil {
		testing.ContextLog(ctx, "Error getting TEST.logs: ", err)
	}
	testing.ContextLog(ctx, "log=", logs)
	return callErr
}
