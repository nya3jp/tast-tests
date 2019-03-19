// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package constants contains values commonly used in video tests.
package constants

const (
	// MediaGVDInitStatus is the name of histogram to describe whether video decoding was hardware-accelerated.
	MediaGVDInitStatus = "Media.GpuVideoDecoderInitializeStatus"

	// MediaGVDInitSuccess is the bucket value in Media.GpuVideoDecoderInitializeStatus to be incremented in success.
	MediaGVDInitSuccess = 0

	// MediaGVDError is the name of histogram used to report video decode errors.
	MediaGVDError = "Media.GpuVideoDecoderError"

	// MediaRecorderVEAUsed is the name of histogram used to report VEA usage when running MediaRecorder.
	MediaRecorderVEAUsed = "Media.MediaRecorder.VEAUsed"
	// MediaRecorderVEAUsedSuccess is the bucket value in MediaRecorderVEAUsed to be incremented in success.
	MediaRecorderVEAUsedSuccess = 1

	// RTCVDInitStatus is the name of histogram used to describe whether HW video decoding is successfully initialized in WebRTC use case.
	RTCVDInitStatus = "Media.RTCVideoDecoderInitDecodeSuccess"
	// RTCVDInitSuccess is the bucket value in Media.RTCVideoDecoderInitDecodeSuccess to be incremented in success.
	RTCVDInitSuccess = 1

	// RTCVEInitStatus is the name of histogram used to describe whether HW video encoding is successfully initialized in WebRTC use case.
	RTCVEInitStatus = "Media.RTCVideoEncoderInitEncodeSuccess"
	// RTCVEInitSuccess is the bucket value in Media.RTCVideoDecoderInitDecodeSuccess to be incremented in success.
	RTCVEInitSuccess = 1
)
