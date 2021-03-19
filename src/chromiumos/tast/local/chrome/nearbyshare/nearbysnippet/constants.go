// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbysnippet

// Constants used in the adb commands for installing and launching the Nearby Snippet.
const (
	ZipName                    = "nearby_snippet.zip"
	ApkName                    = "nearby_snippet.apk"
	protocolVersion            = "1"
	moblyPackage               = "com.google.android.gmscore.integ.modules.nearby.mobly.snippets"
	instrumentationRunnerClass = "com.google.android.mobly.snippet.SnippetRunner"
)

// AccountUtilZip is the filename for the .zip containing the GoogleAccountUtil APK.
const AccountUtilZip = "google_account_util.zip"

// AccountUtilApk is the filename for the GoogleAccountUtil APK.
const AccountUtilApk = "GoogleAccountUtil.apk"

// SendDir is the subdirectory of the Android downloads directory where we will stage files for sending.
const SendDir = "test_files"

// DataUsage are data usage values for the Nearby Snippet's setupDevice and getDataUsage methods.
type DataUsage int

// These are the 3 values defined by the Nearby Snippet API.
const (
	DataUsageOffline DataUsage = iota + 1
	DataUsageOnline
	DataUsageWifiOnly
)

// DataUsageStrings is a map of DataUsage to human-readable setting values.
var DataUsageStrings = map[DataUsage]string{
	DataUsageOffline:  "Offline",
	DataUsageOnline:   "Online",
	DataUsageWifiOnly: "Wifi Only",
}

// Visibility are values for the Nearby Snippet's setupDevice and getVisibility methods, corresponding to different contact visibility settings.
type Visibility int

// These are the 5 values defined by the Nearby Snippet API.
const (
	VisibilityUnknown Visibility = iota - 1
	VisibilityNoOne
	VisibilityAllContacts
	VisibilitySelectedContacts
	VisibilityEveryone
)

// VisibilityStrings is a map of Visibility to human-readable setting values.
var VisibilityStrings = map[Visibility]string{
	VisibilityUnknown:          "Unknown",
	VisibilityNoOne:            "No One",
	VisibilityAllContacts:      "All Contacts",
	VisibilitySelectedContacts: "Selected Contacts",
	VisibilityEveryone:         "Everyone",
}

// SnippetEvent are the event names posted by the Nearby Snippet to its event cache after initiating receiving.
// The host CrOS device can monitor the sharing state by awaiting these events using the Nearby Snippet's eventWaitAndGet RPC.
type SnippetEvent string

// Event names defined by the Nearby Snippet.
const (
	// Snippet events when Android is the receiver.
	SnippetEventOnLocalConfirmation SnippetEvent = "onLocalConfirmation"
	SnippetEventOnReceiveStatus     SnippetEvent = "onReceiveStatus"
	// Snippet events when Android is the sender.
	SnippetEventOnReceiverFound          SnippetEvent = "onReceiverFound"
	SnippetEventOnAwaitingReceiverAccept SnippetEvent = "onAwaitingReceiverAccept"
	SnippetEventOnTransferStatus         SnippetEvent = "onTransferStatus"
	// Shared Snippet event when Android is sender and receiver.
	// The onStop event indicates that the transfer is complete and all teardown tasks for Android Nearby are complete.
	SnippetEventOnStop SnippetEvent = "onStop"
)

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
