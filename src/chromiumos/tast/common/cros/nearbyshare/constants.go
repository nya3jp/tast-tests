// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package nearbyshare is used to control ChromeOS Nearby Share functionality.
package nearbyshare

import (
	"time"

	"chromiumos/tast/common/cros/crossdevice"
)

// DetectShareTargetTimeout is the timeout for a sender to detect an available receiver or vice versa.
// TODO(b:192296468) Decrease this back to a minute after investigation is over.
const DetectShareTargetTimeout = 3 * time.Minute

// SmallFileTransferTimeout is the test timeout for small file (~10kb) transfer tests.
const SmallFileTransferTimeout = 30 * time.Second

// MediumFileOnlineTransferTimeout is the transfer timeout for medium file (~5MB) online transfer tests.
// Online transfers should be at least 10x faster than offline transfers.
// Some extra time is required to account for the delay in upgrading the bandwidth.
const MediumFileOnlineTransferTimeout = 30 * time.Second

// LargeFileOnlineTransferTimeout is the transfer timeout for large file (~30MB) online transfer tests.
const LargeFileOnlineTransferTimeout = time.Minute

// ExtraLargeFileOnlineTransferTimeout is the transfer timeout for extra large file (~100MB) online transfer tests.
const ExtraLargeFileOnlineTransferTimeout = 3 * time.Minute

// AdditionalTestTime is the amount of time to add to the share target detection and file transfer timeouts to make up the global test timeout.
// This is to account for any additional setup and non-sharing interactions performed by the test.
const AdditionalTestTime = 30 * time.Second

// DetectionTimeout is the standard timeout for activities performed in a test excluding the actual file transfer.
// 2*DetectShareTargetTimeout accounts for the amount of time given for the sender to find+select the receiver,
// and then for the receiver to detect the incoming share from the sender.
const DetectionTimeout = 2*DetectShareTargetTimeout + AdditionalTestTime

// ChromeLog is the filename of the Chrome log that is saved for each test.
const ChromeLog = "nearby_chrome"

// MessageLog is the filename of the messages log that is saved for each test. It is saved automatically by tast for local tests. For remote tests we need to grab it within the test.
const MessageLog = "nearby_messages"

// BtsnoopLog is the filename of the ChromeOS btsnoop log that is saved for each test.
const BtsnoopLog = "nearby_btsnoop_cros.log"

// NearbyLogDir is the dir that logs will be saved in temporarily on the DUT during remote tests before being pulled back to remote host.
const NearbyLogDir = "/tmp/nearbyshare/"

// KeepStateVar is the runtime variable name used to specify the chrome.KeepState parameter to preserve the DUT's user accounts.
const KeepStateVar = "keepState"

// MimeType are the mime type values that are accepted by the snippet's sendFile method.
type MimeType string

// MimeTypes supported by the snippet library.
const (
	MimeTypeTextVCard MimeType = "text/x-vcard"
	MimeTypePDF       MimeType = "application/pdf"
	MimeTypeJpeg      MimeType = "image/jpeg"
	MimeTypeMP4       MimeType = "video/mp4"
	MimeTypeTextPlain MimeType = "text/plain"
	MimeTypePng       MimeType = "image/png"
)

// SecurityType is the Wi-Fi network security type that is expected by the snippet's SendWifi method.
type SecurityType string

// SecurityTypes supported by the snippet library.
const (
	SecurityTypeOpen    SecurityType = "Open"
	SecurityTypeUnknown SecurityType = "Unknown"
	SecurityTypeWpaPsk  SecurityType = "WpaPsk"
	SecurityTypeWep     SecurityType = "Wep"
)

// TestData contains the values for parameterized tests, such as:
// - File name of the archive containing files to be shared
// - File transfer timeout (varies depending on file size)
// - Total test timeout (transfer timeout + time required for sender and receiver to detect each other)
// - MIME type of shared files (only required when sending from Android)
type TestData struct {
	Filename        string
	TransferTimeout time.Duration
	TestTimeout     time.Duration
	MimeType        MimeType
}

// WiFiTestData contains values for parameterized tests for Wi-Fi credentials:
// - Wi-Fi name / SSID / Network name
// - Wi-Fi password
// - Wi-Fi transfer timeout
// - Total test timeout (transfer timeout + time required for sender and receiver to detect each other)
// - Security type of the Wi-Fi network
type WiFiTestData struct {
	WiFiName        string
	WiFiPassword    string
	TransferTimeout time.Duration
	TestTimeout     time.Duration
	SecurityType    SecurityType
}

// DownloadPath is the downloads directory on CrOS.
const DownloadPath = "/home/chronos/user/Downloads/"

// SendDir is the staging directory for test files when sending from CrOS.
const SendDir = DownloadPath + "nearby_test_files"

// CrosAttributes contains information about the CrOS device that are relevant to Nearby Share.
type CrosAttributes struct {
	BasicAttributes *crossdevice.CrosAttributes
	DisplayName     string
	DataUsage       string
	Visibility      string
}

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
