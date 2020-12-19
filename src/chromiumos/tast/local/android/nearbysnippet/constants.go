// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbysnippet

// NearbySnippetZip is the file name of the zipped APK that the tests pull down from Google Storage.
const NearbySnippetZip = "nearby_snippet_20201217.zip"

// NearbySnippetApk is the file name of the APK.
const NearbySnippetApk = "nearby_snippet.apk"

// NearbySnippetProtocolVersion is the expected protocol version of the Nearby snippet.
const nearbySnippetProtocolVersion = "1"

// NearbySnippetPackage is the Java package name.
const nearbySnippetPackage = "com.google.android.gmscore.integ.modules.nearby.mobly.snippets"

// AndroidDefaultUser is the default Android user ID, corresponding to the system user that adb runs as.
const androidDefaultUser = "0"

// InstrumentationRunnerPackage is the instrumentation runner to run the snippet.
const instrumentationRunnerPackage = "com.google.android.mobly.snippet.SnippetRunner"

// downloadDir is the download directory where incoming shares go.
const downloadDir = "/sdcard/Download/"

// DataUsage are data usage values for the snippet's setupDevice and getDataUsage methods.
type DataUsage int

// These are the 3 values defined by the snippet server API.
const (
	DataUsageOffline DataUsage = iota + 1
	DataUsageOnline
	DataUsageWifiOnly
)

// Visibility are values for the snippet's setupDevice and getVisibility methods, corresponding to different contact visibility settings.
type Visibility int

// These are the 3 values defined by the snippet server API.
const (
	VisibilityUnknown Visibility = iota - 1
	VisibilityNoOne
	VisibilityAllContacts
	VisibilitySelectedContacts
	VisibilityEveryone
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
