// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package videotype defines types and values commonly used in video tests.
package videotype

// Codec describes a video codec to exercise in testing.
// String values are passed to testWebRtcLoopbackCall in /video/data/loopback.html
// and cannot be changed. Also, they are used in field names in
// WebRTCPeerConnectionWithCamera tests' performance metrics.
type Codec string

const (
	// VP8 represents the VP8 codec.
	VP8 Codec = "VP8"
	// H264 represents the H.264 codec.
	H264 Codec = "H264"
)
