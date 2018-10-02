// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package videotype defines types and values commonly used in video tests.
package videotype

// PixelFormat stands for pixel format in yuv image.
type PixelFormat int

const (
	// These values must match integers in VideoPixelFormat in https://cs.chromium.org/chromium/src/media/base/video_types.h
	I420 PixelFormat = 1
	NV12 PixelFormat = 6
)

// CodecProfile stands for a profile in video.
type CodecProfile int

const (
	// These values must match integers in VideoCodecProfile in https://cs.chromium.org/chromium/src/media/base/video_codecs.h
	H264  CodecProfile = 1  // = H264PROFILE_MAIN
	VP8   CodecProfile = 11 // = VP8PROFILE_ANY
	VP9   CodecProfile = 12 // = VP9PROFILE_PROFILE0
	VP9_2 CodecProfile = 14 // = VP9PROFILE_PROFILE2
)
