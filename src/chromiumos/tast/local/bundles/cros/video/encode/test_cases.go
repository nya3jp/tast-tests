// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package encode

import "chromiumos/tast/local/bundles/cros/video/lib/videotype"

// BearI420 is the test parameters of video_encode_accelerator_unittest for "bear_320x192_40frames.yuv".
var BearI420 = StreamParams{
	Name:    "bear_320x192_40frames.yuv",
	Width:   320,
	Height:  192,
	Bitrate: 200000,
	Format:  videotype.I420,
}

// BearNV12 is the test parameters of video_encode_accelerator_unittest for "bear_320x192_40frames.nv12.yuv".
var BearNV12 = StreamParams{
	Name:    "bear_320x192_40frames.nv12.yuv",
	Width:   320,
	Height:  192,
	Bitrate: 200000,
	Format:  videotype.NV12,
}

// Crowd1080PI420 is the test parameters of video_encode_accelerator_unittest for "crowd1920x1080.yuv".
var Crowd1080PI420 = StreamParams{
	Name:    "crowd-1920x1080.yuv",
	Width:   1920,
	Height:  1080,
	Bitrate: 4000000,
	Format:  videotype.I420,
}

// Tulip720PI420 is the test parameters of video_encode_accelerator_unittest for "tulip2-1280x720.yuv".
var Tulip720PI420 = StreamParams{
	Name:    "tulip2-1280x720.yuv",
	Width:   1280,
	Height:  720,
	Bitrate: 1200000,
	Format:  videotype.I420,
}

// Tulip360PI420 is the test parameters of video_encode_accelerator_unittest for "tulip2-640x360.yuv".
var Tulip360PI420 = StreamParams{
	Name:    "tulip2-640x360.yuv",
	Width:   640,
	Height:  360,
	Bitrate: 500000,
	Format:  videotype.I420,
}

// Tulip180PI420 is the test parameters of video_encode_accelerator_unittest for "tulip2-320x180.yuv".
var Tulip180PI420 = StreamParams{
	Name:    "tulip2-320x180.yuv",
	Width:   320,
	Height:  180,
	Bitrate: 100000,
	Format:  videotype.I420,
}
