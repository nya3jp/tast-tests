// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbyshare

import "chromiumos/tast/local/chrome/ui"

var receiveUIParams ui.FindParams = ui.FindParams{
	Role: ui.RoleTypeRootWebArea,
	Name: "Settings - Nearby Share",
}

// CrOSDataUsage represents Nearby Share data usage setting values.
type CrOSDataUsage int

// As defined in https://chromium.googlesource.com/chromium/src/+/master/chrome/browser/ui/webui/nearby_share/public/mojom/nearby_share_settings.mojom
const (
	CrOSDataUsageUnknown  CrOSDataUsage = 0
	CrOSDataUsageOffline  CrOSDataUsage = 1
	CrOSDataUsageOnline   CrOSDataUsage = 2
	CrOSDataUsageWifiOnly CrOSDataUsage = 3
)

// CrOSVisibility represents Nearby Share visibility setting values.
type CrOSVisibility int

// As defined in https://chromium.googlesource.com/chromium/src/+/master/chrome/browser/ui/webui/nearby_share/public/mojom/nearby_share_settings.mojom
const (
	CrOSVisibilityUnknown          CrOSVisibility = 0
	CrOSVisibilityNoOne            CrOSVisibility = 1
	CrOSVisibilityAllContacts      CrOSVisibility = 2
	CrOSVisibilitySelectedContacts CrOSVisibility = 3
)

// CrOSDeviceNameValidationResult represents device name validation results that are returned after setting the device name programatically.
type CrOSDeviceNameValidationResult int

// As defined in https://chromium.googlesource.com/chromium/src/+/master/chrome/browser/ui/webui/nearby_share/public/mojom/nearby_share_settings.mojom
const (
	CrOSDeviceNameValidationResultValid             CrOSDeviceNameValidationResult = 0
	CrOSDeviceNameValidationResultErrorEmpty        CrOSDeviceNameValidationResult = 1
	CrOSDeviceNameValidationResultErrorTooLong      CrOSDeviceNameValidationResult = 2
	CrOSDeviceNameValidationResultErrorNotValidUtf8 CrOSDeviceNameValidationResult = 3
)
