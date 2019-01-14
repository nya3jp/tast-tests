// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package decode

import "chromiumos/tast/local/bundles/cros/video/lib/videotype"

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

// DecodeAccelSanityVP91 is the test parameters of video_decode_accelerator_unittest for "vda_sanity-bear_profile1.vp9".
var DecodeAccelSanityVP91 = TestVideoData{
	Name:             "vda_sanity-bear_profile1.vp9",
	Size:             videotype.NewSize(320, 180),
	NumFrames:        30,
	NumFragments:     30,
	MinFPSWithRender: 0,
	MinFPSNoRender:   0,
	Profile:          videotype.VP9Prof,
}

// DecodeAccelSanityVP92 is the test parameters of video_decode_accelerator_unittest for "vda_sanity-bear_profile2.vp9".
var DecodeAccelSanityVP92 = TestVideoData{
	Name:             "vda_sanity-bear_profile2.vp9",
	Size:             videotype.NewSize(320, 180),
	NumFrames:        30,
	NumFragments:     30,
	MinFPSWithRender: 0,
	MinFPSNoRender:   0,
	Profile:          videotype.VP9Prof,
}

// DecodeAccelSanityVP93 is the test parameters of video_decode_accelerator_unittest for "vda_sanity-bear_profile3.vp9".
var DecodeAccelSanityVP93 = TestVideoData{
	Name:             "vda_sanity-bear_profile3.vp9",
	Size:             videotype.NewSize(320, 180),
	NumFrames:        30,
	NumFragments:     30,
	MinFPSWithRender: 0,
	MinFPSNoRender:   0,
	Profile:          videotype.VP9Prof,
}

// DecodeAccelSanityVP90CtsShowExistingFrame is the test parameters of
// video_decode_accelerator_unittest for "vda_sanity-vp90_2_17_show_existing_frame.vp9".
// It is from Android CTS:
// https://android.googlesource.com/platform/cts/+/master/tests/tests/media/res/raw/vp90_2_17_show_existing_frame.vp9
// which causes elm kernel crash (crbug.com/900467).
var DecodeAccelSanityVP90CtsShowExistingFrame = TestVideoData{
	Name:             "vda_sanity-vp90_2_17_show_existing_frame.vp9",
	Size:             videotype.NewSize(352, 288),
	NumFrames:        30,
	NumFragments:     30,
	MinFPSWithRender: 0,
	MinFPSNoRender:   0,
	Profile:          videotype.VP9Prof,
}
