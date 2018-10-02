// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package constants is a package for constant values commonly used in package video.
package constants

// These constant values are commonly used in video tests.
const (
	// MediaGVDInitStatus is the histogram name for an initialization result
	// in Chrome video decode accelerators.
	MediaGVDInitStatus = "Media.GpuVideoDecoderInitializeStatus"

	// MediaGVDBucket is the bucket value in Media.GpuVideoDecoderInitializeStatus
	// to be incremented in success.
	MediaGVDBucket = 0

	// MediaGVDError is the histogram name for an error status in
	// Chrome video decode accelerators.
	MediaGVDError = "Media.GpuVideoDecoderError"
)
