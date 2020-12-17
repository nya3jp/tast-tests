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

// NearbySnippetApk is the file name of the APK.
const NearbySnippetApk = "nearby_snippet_20201217.apk"

// NearbySnippetProtocolVersion is the expected protocol version of the Nearby snippet.
const nearbySnippetProtocolVersion = "1"

// NearbySnippetPackage is the Java package name.
const nearbySnippetPackage = "com.google.android.gmscore.integ.modules.nearby.mobly.snippets"

// AndroidDefaultUser is the default Android user ID.
const androidDefaultUser = "0"

// InstrumentationRunnerPackage is the instrumentation runner to run the snippet.
const instrumentationRunnerPackage = "com.google.android.mobly.snippet.SnippetRunner"

// SnippetDataUsage are data usage values for the snippet's setupDevice and getDataUsage methods.
type SnippetDataUsage int

// These are the 3 values defined by the snippet server API.
const (
	SnippetDataUsageOffline  SnippetDataUsage = 1
	SnippetDataUsageOnline   SnippetDataUsage = 2
	SnippetDataUsageWifiOnly SnippetDataUsage = 3
)

// SnippetVisibility are values for the snippet's setupDevice and getVisibility methods, corresponding to different contact visibility settings.
type SnippetVisibility int

// These are the 3 values defined by the snippet server API.
const (
	SnippetVisibilityUnknown          SnippetVisibility = -1
	SnippetVisibilityNoOne            SnippetVisibility = 0
	SnippetVisibilityAllContacts      SnippetVisibility = 1
	SnippetVisibilitySelectedContacts SnippetVisibility = 2
	SnippetVisibilityEveryone         SnippetVisibility = 3
)

// SnippetEvent are the event names posted by the snippet server after initiating receiving.
type SnippetEvent string

// Snippet events when Android is the receiver.
const (
	SnippetEventOnLocalConfirmation SnippetEvent = "onLocalConfirmation"
	SnippetEventOnReceiveStatus     SnippetEvent = "onReceiveStatus"
)

// Snippet events when Android is the sender.
const (
	SnippetEventOnReceiverFound          SnippetEvent = "onReceiverFound"
	SnippetEventOnAwaitingReceiverAccept SnippetEvent = "onAwaitingReceiverAccept"
	SnippetEventOnTransferStatus         SnippetEvent = "onTransferStatus"
)

// Shared Snippet events.
const (
	SnippetEventOnStop SnippetEvent = "onStop"
)
