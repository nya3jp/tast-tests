// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package simpleclient provides const log patterns used in cros/hardware
// package.
package simpleclient

const (
	// OnErrorOccurred is the error log in iioservice_simpleclient.
	OnErrorOccurred = "OnErrorOccurred:"

	// SucceedReadingSamples is the successful read log in
	// iioservice_simpleclient.
	SucceedReadingSamples = "Number of success reads"
)
