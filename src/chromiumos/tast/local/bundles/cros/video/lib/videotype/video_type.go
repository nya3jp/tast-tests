// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package videotype defines types and values commonly used in video tests.
package videotype

// Codec describes a video codec to exercise in testing.
// String values are passed to testWebRtcLoopbackCall in /video/data/loopback_camera.html
// and cannot be changed. Also, they are used in field names in
// WebRTCPeerConnCamera tests' performance metrics.
type Codec string

const (
	// VP8 represents the VP8 codec.
	VP8 Codec = "VP8"
	// VP9 represents the VP8 codec.
	VP9 Codec = "VP9"
	// H264 represents the H.264 codec.
	H264 Codec = "H264"
)

// PixelFormat stands for pixel format in yuv image.
type PixelFormat int

const (
	// These values must match integers in VideoPixelFormat in https://cs.chromium.org/chromium/src/media/base/video_types.h

	// I420 represents the value for the I420 pixel format (= PIXEL_FORMAT_I420).
	I420 PixelFormat = 1
	// NV12 represents the value for the NV12 pixel format (= PIXEL_FORMAT_NV12).
	NV12 PixelFormat = 6
)

// CodecProfile stands for a profile in video.
type CodecProfile int

const (
	// These values must match integers in VideoCodecProfile in https://cs.chromium.org/chromium/src/media/base/video_codecs.h

	// H264Prof represents the value for H264 Main profile (= H264PROFILE_MAIN).
	H264Prof CodecProfile = 1
	// VP8Prof represents the value for VP8 Main profile (= VP8PROFILE_ANY).
	VP8Prof CodecProfile = 11
	// VP9Prof represents the value for VP9 profile 0 (= VP9PROFILE_PROFILE0).
	VP9Prof CodecProfile = 12
	// VP9_2Prof represents the value for VP9 profile 2 (= VP9PROFILE_PROFILE2).
	VP9_2Prof CodecProfile = 14
)

// Size composes (width, height).
type Size struct {
	// W, H stands for width and height, respectively.
	W, H int
}

// NewSize creates Size from w and h.
func NewSize(w, h int) Size { return Size{W: w, H: h} }
