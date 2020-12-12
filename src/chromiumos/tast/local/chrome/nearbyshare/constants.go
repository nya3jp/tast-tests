// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbyshare

import "chromiumos/tast/local/chrome/ui"

var receiveUIParams ui.FindParams = ui.FindParams{
	Role: ui.RoleTypeRootWebArea,
	Name: "Settings - Nearby Share",
}

var confirmBtnParams ui.FindParams = ui.FindParams{
	Role: ui.RoleTypeButton,
	Name: "Confirm",
}

var incomingShareParams ui.FindParams = ui.FindParams{
	Role: ui.RoleTypeStaticText,
	Name: "Receive from this device?",
}

// SharingContentType is the type of data for sharing.
// These strings will be used to verify that a share was received by checking for the text in the receipt notification.
type SharingContentType string

// List of sharing content types - these appear in the text of the notification after a share is received.
const (
	SharingContentLink SharingContentType = "link"
	SharingContentFile SharingContentType = "file"
)

// ReceivedFollowUpMap maps the SharingContentType to the text of the suggested follow-up action in the receipt notification.
var ReceivedFollowUpMap = map[SharingContentType]string{
	SharingContentLink: "COPY TO CLIPBOARD",
	SharingContentFile: "OPEN FOLDER",
}

// NearbySnippetApk is the file name of the APK.
const NearbySnippetApk = "nearby_snippet.apk"

// NearbySnippetProtocolVersion is the expected protocol version of the Nearby snippet.
const NearbySnippetProtocolVersion = "1"

// NearbySnippetPackage is the java package name.
const NearbySnippetPackage = "com.google.android.gmscore.integ.modules.nearby.mobly.snippets"

// AndroidDefaultUser is the default Android user ID.
const AndroidDefaultUser = "0"

// InstrumentationRunnerPackage is the instrumentation runner to run the snippet.
const InstrumentationRunnerPackage = "com.google.android.mobly.snippet.SnippetRunner"

// SnippetDataUsage are data usage values for the snippet's setupDevice and getDataUsage methods.
type SnippetDataUsage int

// These are the 3 values defined by the snippet server API.
const (
	SnippetDataUsageOffline  = 1
	SnippetDataUsageOnline   = 2
	SnippetDataUsageWifiOnly = 3
)

// SnippetVisibility are values for the snippet's setupDevice and getVisibility methods, corresponding to different contact visibility settings.
type SnippetVisibility int

// These are the 3 values defined by the snippet server API.
const (
	SnippetVisibilityUnknown  = -1
	SnippetVisibilityNoOne  = 0
	SnippetVisibilityAllContacts  = 1
	SnippetVisibilitySelectedContacts   = 2
	SnippetVisibilityEveryone = 3
)

// SnippetEvent are the event names posted by the snippet server after initiating receiving.
type SnippetEvent string
const (
	// Receiver events.
	SnippetEventOnLocalConfirmation = "onLocalConfirmation"
	SnippetEventOnReceiveStatus = "onReceiveStatus"
	// Sender events.
	SnippetEventOnReceiverFound = "onReceiverFound"
	SnippetEventOnAwaitingReceiverAccept = "onAwaitingReceiverAccept"
	SnippetEventOnTransferStatus = "onTransferStatus"
	// Shared.
	SnippetEventOnStop = "onStop"
)