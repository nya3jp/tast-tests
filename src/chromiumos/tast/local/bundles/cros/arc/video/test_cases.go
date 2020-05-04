// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/media/encoding"
)

// Bear192P is the test parameters of video_encode_accelerator_unittest for "bear_320x192_40frames.yuv".
var Bear192P = encoding.StreamParams{
	Name:    "bear-320x192.vp9.webm",
	Size:    coords.NewSize(320, 192),
	Bitrate: 200000,
}

// Tulip360P is the test parameters of video_encode_accelerator_unittest for the raw data obtained by decoding "tulip2-640x360.webm".
var Tulip360P = encoding.StreamParams{
	Name:    "tulip2-640x360.vp9.webm",
	Size:    coords.NewSize(640, 360),
	Bitrate: 500000,
}

// Tulip720P is the test parameters of video_encode_accelerator_unittest for the raw data obtained by decoding "tulip2-1280x720.webm".
var Tulip720P = encoding.StreamParams{
	Name:    "tulip2-1280x720.vp9.webm",
	Size:    coords.NewSize(1280, 720),
	Bitrate: 1200000,
}

// Crowd1080P is the test parameters of video_encode_accelerator_unittest for the raw data obtained by decoding "crowd1920x1080.webm".
var Crowd1080P = encoding.StreamParams{
	Name:    "crowd-1920x1080.vp9.webm",
	Size:    coords.NewSize(1920, 1080),
	Bitrate: 4000000,
}
