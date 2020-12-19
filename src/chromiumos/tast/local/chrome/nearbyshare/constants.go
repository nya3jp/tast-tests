// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package nearbyshare is used to control Chrome OS Nearby Share functionality.
package nearbyshare

import (
	"time"

	"chromiumos/tast/local/chrome/ui"
)

// ReceiveUIParams are the UI FindParams for the receiving UI's root node.
var ReceiveUIParams ui.FindParams = ui.FindParams{
	Role: ui.RoleTypeRootWebArea,
	Name: "Settings - Nearby Share",
}

// DataUsage represents Nearby Share data usage setting values.
type DataUsage int

// As defined in https://chromium.googlesource.com/chromium/src/+/master/chrome/browser/ui/webui/nearby_share/public/mojom/nearby_share_settings.mojom
const (
	DataUsageUnknown DataUsage = iota
	DataUsageOffline
	DataUsageOnline
	DataUsageWifiOnly
)

// Visibility represents Nearby Share visibility setting values.
type Visibility int

// As defined in https://chromium.googlesource.com/chromium/src/+/master/chrome/browser/ui/webui/nearby_share/public/mojom/nearby_share_settings.mojom
const (
	VisibilityUnknown Visibility = iota
	VisibilityNoOne
	VisibilityAllContacts
	VisibilitySelectedContacts
)

// DeviceNameValidationResult represents device name validation results that are returned after setting the device name programmatically.
type DeviceNameValidationResult int

// As defined in https://chromium.googlesource.com/chromium/src/+/master/chrome/browser/ui/webui/nearby_share/public/mojom/nearby_share_settings.mojom
const (
	DeviceNameValidationResultValid DeviceNameValidationResult = iota
	DeviceNameValidationResultErrorEmpty
	DeviceNameValidationResultErrorTooLong
	DeviceNameValidationResultErrorNotValidUtf8
)

// SendDir is the staging directory for test files when sending from CrOS.
const SendDir = "/home/chronos/user/Downloads/nearby_test_files"

// CrosDetectReceiverTimeout is the timeout for a CrOS sender to detect a receiver.
const CrosDetectReceiverTimeout = time.Minute

// SmallFileTimeout is the test timeout for small file transfer tests.
const SmallFileTimeout = 2 * time.Minute
