// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package enc

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
