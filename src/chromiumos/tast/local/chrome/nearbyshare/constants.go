// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbyshare

import "chromiumos/tast/local/chrome/ui"

// receiveUIParams are the UI FindParams for the receiving UI's root node.
var receiveUIParams ui.FindParams = ui.FindParams{
	Role: ui.RoleTypeRootWebArea,
	Name: "Settings - Nearby Share",
}

// CrOSDataUsage represents Nearby Share data usage setting values.
type CrOSDataUsage int

// As defined in https://chromium.googlesource.com/chromium/src/+/master/chrome/browser/ui/webui/nearby_share/public/mojom/nearby_share_settings.mojom
const (
	CrOSDataUsageUnknown CrOSDataUsage = iota
	CrOSDataUsageOffline
	CrOSDataUsageOnline
	CrOSDataUsageWifiOnly
)

// CrOSVisibility represents Nearby Share visibility setting values.
type CrOSVisibility int

// As defined in https://chromium.googlesource.com/chromium/src/+/master/chrome/browser/ui/webui/nearby_share/public/mojom/nearby_share_settings.mojom
const (
	CrOSVisibilityUnknown CrOSVisibility = iota
	CrOSVisibilityNoOne
	CrOSVisibilityAllContacts
	CrOSVisibilitySelectedContacts
)

// CrOSDeviceNameValidationResult represents device name validation results that are returned after setting the device name programmatically.
type CrOSDeviceNameValidationResult int

// As defined in https://chromium.googlesource.com/chromium/src/+/master/chrome/browser/ui/webui/nearby_share/public/mojom/nearby_share_settings.mojom
const (
	CrOSDeviceNameValidationResultValid CrOSDeviceNameValidationResult = iota
	CrOSDeviceNameValidationResultErrorEmpty
	CrOSDeviceNameValidationResultErrorTooLong
	CrOSDeviceNameValidationResultErrorNotValidUtf8
)
