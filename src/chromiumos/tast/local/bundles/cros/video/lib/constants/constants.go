// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package constants contains values commonly used in video tests.
package constants

const (
	// MediaGVDInitStatus is the name of histogram describing whether video decoding was hardware-accelerated.
	MediaGVDInitStatus = "Media.GpuVideoDecoderInitializeStatus"

	// MediaGVDBucket is the bucket value in Media.GpuVideoDecoderInitializeStatus to be incremented in success.
	MediaGVDInitSuccess = 0

	// MediaGVDError is the name of histogram used to report video decode errors.
	MediaGVDError = "Media.GpuVideoDecoderError"
)
