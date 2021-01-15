// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package encode

import (
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/media/encoding"
)

// Bear192P is the test parameters of video_encode_accelerator_tests for "bear_320x192_40frames.yuv".
var Bear192P = encoding.StreamParams{
	Name:    "bear-320x192.vp9.webm",
	Size:    coords.NewSize(320, 192),
	Bitrate: 200000,
}

// Crowd1080P is the test parameters of video_encode_accelerator_tests for the raw data obtained by decoding "crowd1920x1080.webm".
var Crowd1080P = encoding.StreamParams{
	Name:    "crowd-1920x1080.vp9.webm",
	Size:    coords.NewSize(1920, 1080),
	Bitrate: 4000000,
}

// Tulip720P is the test parameters of video_encode_accelerator_tests for the raw data obtained by decoding "tulip2-1280x720.webm".
var Tulip720P = encoding.StreamParams{
	Name:    "tulip2-1280x720.vp9.webm",
	Size:    coords.NewSize(1280, 720),
	Bitrate: 1200000,
}

// Vidyo720P is the test parameters of video_encode_accelerator_tests for the raw data obtained by decoding "vidyo1-1280x720.webm".
var Vidyo720P = encoding.StreamParams{
	Name:    "vidyo1-1280x720.vp9.webm",
	Size:    coords.NewSize(1280, 720),
	Bitrate: 1200000,
}

// Tulip360P is the test parameters of video_encode_accelerator_tests for the raw data obtained by decoding "tulip2-640x360.webm".
var Tulip360P = encoding.StreamParams{
	Name:    "tulip2-640x360.vp9.webm",
	Size:    coords.NewSize(640, 360),
	Bitrate: 500000,
}

// Tulip180P is the test parameters of video_encode_accelerator_tests for the raw data obtained by decoding "tulip2-320x180.webm".
var Tulip180P = encoding.StreamParams{
	Name:    "tulip2-320x180.vp9.webm",
	Size:    coords.NewSize(320, 180),
	Bitrate: 100000,
}

// Crowd2160P is the test parameters of video_encode_accelerator_tests for the raw data obtained by decoding "crowd-3840x2160.webm". The H264 level needs to be 51 (AVC Level5.1) for the capability of 2160p@30FPS encoding.
var Crowd2160P = encoding.StreamParams{
	Name:    "crowd-3840x2160.vp9.webm",
	Size:    coords.NewSize(3840, 2160),
	Bitrate: 20000000,
	Level:   51,
}

// Crowd361P is the test parameters of video_encode_accelerator_tests for the raw data obtained by decoding "crowd-641x361.webm".
var Crowd361P = encoding.StreamParams{
	Name:    "crowd-641x361.vp9.webm",
	Size:    coords.NewSize(641, 361),
	Bitrate: 500000,
}
