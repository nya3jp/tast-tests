// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbysnippet

// Constants used in the adb commands for installing and launching the Nearby Snippet.
const (
	ZipName                      = "nearby_snippet.zip"
	ApkName                      = "nearby_snippet.apk"
	protocolVersion              = "1"
	moblyPackage                 = "com.google.android.gmscore.integ.modules.nearby.mobly.snippets"
	instrumentationRunnerPackage = "com.google.android.mobly.snippet.SnippetRunner"
)

// DataUsage are data usage values for the Nearby Snippet's setupDevice and getDataUsage methods.
type DataUsage int

// These are the 3 values defined by the Nearby Snippet API.
const (
	DataUsageOffline DataUsage = iota + 1
	DataUsageOnline
	DataUsageWifiOnly
)

// Visibility are values for the Nearby Snippet's setupDevice and getVisibility methods, corresponding to different contact visibility settings.
type Visibility int

// These are the 3 values defined by the Nearby Snippet API.
const (
	VisibilityUnknown Visibility = iota - 1
	VisibilityNoOne
	VisibilityAllContacts
	VisibilitySelectedContacts
	VisibilityEveryone
)

// SnippetEvent are the event names posted by the Nearby Snippet after initiating receiving.
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
