// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package encode

import "chromiumos/tast/local/media/videotype"

// Bear192P is the test parameters of video_encode_accelerator_unittest for "bear_320x192_40frames.yuv".
var Bear192P = StreamParams{
	Name:    "bear-320x192.vp9.webm",
	Size:    videotype.NewSize(320, 192),
	Bitrate: 200000,
}

// Crowd1080P is the test parameters of video_encode_accelerator_unittest for the raw data obtained by decoding "crowd1920x1080.webm".
var Crowd1080P = StreamParams{
	Name:    "crowd-1920x1080.vp9.webm",
	Size:    videotype.NewSize(1920, 1080),
	Bitrate: 4000000,
}

// Tulip720P is the test parameters of video_encode_accelerator_unittest for the raw data obtained by decoding "tulip2-1280x720.webm".
var Tulip720P = StreamParams{
	Name:    "tulip2-1280x720.vp9.webm",
	Size:    videotype.NewSize(1280, 720),
	Bitrate: 1200000,
}

// Vidyo720P is the test parameters of video_encode_accelerator_unittest for the raw data obtained by decoding "vidyo1-1280x720.webm".
var Vidyo720P = StreamParams{
	Name:    "vidyo1-1280x720.vp9.webm",
	Size:    videotype.NewSize(1280, 720),
	Bitrate: 1200000,
}

// Tulip360P is the test parameters of video_encode_accelerator_unittest for the raw data obtained by decoding "tulip2-640x360.webm".
var Tulip360P = StreamParams{
	Name:    "tulip2-640x360.vp9.webm",
	Size:    videotype.NewSize(640, 360),
	Bitrate: 500000,
}

// Tulip180P is the test parameters of video_encode_accelerator_unittest for the raw data obtained by decoding "tulip2-320x180.webm".
var Tulip180P = StreamParams{
	Name:    "tulip2-320x180.vp9.webm",
	Size:    videotype.NewSize(320, 180),
	Bitrate: 100000,
}

// Crowd2160P is the test parameters of video_encode_accelerator_unittest for the raw data obtained by decoding "crowd-3840x2160.webm". The H264 level needs to be 51 (AVC Level5.1) for the capability of 2160p@30FPS encoding.
var Crowd2160P = StreamParams{
	Name:    "crowd-3840x2160.vp9.webm",
	Size:    videotype.NewSize(3840, 2160),
	Bitrate: 20000000,
	Level:   51,
}

// Crowd361P is the test parameters of video_encode_accelerator_unittest for the raw data obtained by decoding "crowd-641x361.webm".
var Crowd361P = StreamParams{
	Name:    "crowd-641x361.vp9.webm",
	Size:    videotype.NewSize(641, 361),
	Bitrate: 500000,
}

// BitrateTestFilter is the test pattern in googletest style for disabling bitrate control related tests.
const BitrateTestFilter = "-MidStreamParamSwitchBitrate/*:ForceBitrate/*:MultipleEncoders/VideoEncodeAcceleratorTest.TestSimpleEncode/1"
