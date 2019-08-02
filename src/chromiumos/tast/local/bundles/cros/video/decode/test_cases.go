// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package decode

import "chromiumos/tast/local/media/videotype"

// Test25FPSH264 is the test parameters of video_decode_accelerator_unittest for "test-25fps.h264".
var Test25FPSH264 = TestVideoData{
	Name:             "test-25fps.h264",
	Size:             videotype.NewSize(320, 240),
	NumFrames:        250,
	NumFragments:     258,
	MinFPSWithRender: 35,
	MinFPSNoRender:   150,
	Profile:          videotype.H264Prof,
}

// Test25FPSVP8 is the test parameters of video_decode_accelerator_unittest for "test-25fps.vp8".
var Test25FPSVP8 = TestVideoData{
	Name:             "test-25fps.vp8",
	Size:             videotype.NewSize(320, 240),
	NumFrames:        250,
	NumFragments:     250,
	MinFPSWithRender: 35,
	MinFPSNoRender:   150,
	Profile:          videotype.VP8Prof,
}

// Test25FPSVP9 is the test parameters of video_decode_accelerator_unittest for "test-25fps.vp9".
var Test25FPSVP9 = TestVideoData{
	Name:             "test-25fps.vp9",
	Size:             videotype.NewSize(320, 240),
	NumFrames:        250,
	NumFragments:     250,
	MinFPSWithRender: 35,
	MinFPSNoRender:   150,
	Profile:          videotype.VP9Prof,
}

// Test25FPSVP92 is the test parameters of video_decode_accelerator_unittest for "test-25fps.vp9_2".
var Test25FPSVP92 = TestVideoData{
	Name:             "test-25fps.vp9_2",
	Size:             videotype.NewSize(320, 240),
	NumFrames:        250,
	NumFragments:     250,
	MinFPSWithRender: 35,
	MinFPSNoRender:   150,
	Profile:          videotype.VP9_2Prof,
}
