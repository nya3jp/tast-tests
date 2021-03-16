// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbysetup

// DataUsage represents Nearby Share data usage setting values.
type DataUsage int

// As defined in https://chromium.googlesource.com/chromium/src/+/HEAD/chrome/browser/ui/webui/nearby_share/public/mojom/nearby_share_settings.mojom
const (
	DataUsageUnknown DataUsage = iota
	DataUsageOffline
	DataUsageOnline
	DataUsageWifiOnly
)

// DataUsageStrings is a map of DataUsage to human-readable setting values.
var DataUsageStrings = map[DataUsage]string{
	DataUsageUnknown:  "Unknown",
	DataUsageOffline:  "Offline",
	DataUsageOnline:   "Online",
	DataUsageWifiOnly: "Wifi Only",
}

// Visibility represents Nearby Share visibility setting values.
type Visibility int

// As defined in https://chromium.googlesource.com/chromium/src/+/HEAD/chrome/browser/ui/webui/nearby_share/public/mojom/nearby_share_settings.mojom
const (
	VisibilityUnknown Visibility = iota
	VisibilityNoOne
	VisibilityAllContacts
	VisibilitySelectedContacts
)

// VisibilityStrings is a map of Visibility to human-readable setting values.
var VisibilityStrings = map[Visibility]string{
	VisibilityUnknown:          "Unknown",
	VisibilityNoOne:            "No One",
	VisibilityAllContacts:      "All Contacts",
	VisibilitySelectedContacts: "Selected Contacts",
}

// DeviceNameValidationResult represents device name validation results that are returned after setting the device name programmatically.
type DeviceNameValidationResult int

// As defined in https://chromium.googlesource.com/chromium/src/+/HEAD/chrome/browser/ui/webui/nearby_share/public/mojom/nearby_share_settings.mojom
const (
	DeviceNameValidationResultValid DeviceNameValidationResult = iota
	DeviceNameValidationResultErrorEmpty
	DeviceNameValidationResultErrorTooLong
	DeviceNameValidationResultErrorNotValidUtf8
)
