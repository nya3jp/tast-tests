// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crossdevice

// Constants used in the adb commands for installing and launching the Multidevice Snippet.
const (
	MultideviceSnippetZipName      = "multidevice_snippet.zip"
	MultideviceSnippetApkName      = "multidevice_snippet.apk"
	MultideviceSnippetMoblyPackage = "com.google.android.gmscore.integ.modules.auth.proximity.mobly.snippet"
)

// AccountUtilZip is the filename for the .zip containing the GoogleAccountUtil APK.
const AccountUtilZip = "google_account_util.zip"

// AccountUtilApk is the filename for the GoogleAccountUtil APK.
const AccountUtilApk = "GoogleAccountUtil.apk"

// KeepStateVar is the runtime variable name used to specify the chrome.KeepState parameter to preserve the DUT's user accounts.
const KeepStateVar = "keepState"

// Feature defines the Cross Device feature we are testing.
type Feature struct {
	Name       FeatureName
	SubFeature SubFeature
}

// FeatureName is the name of the Cross Device feature to test.
type FeatureName int

const (
	// SmartLock defines Smart Lock
	SmartLock FeatureName = iota
	// PhoneHub defines Phone Hub
	PhoneHub
	// NearbyShare defines Nearby Share
	NearbyShare
	// Exo defines Exo
	Exo
)

// SubFeature is the specific part of a feature we are testing.
type SubFeature int

const (
	// SmartLockUnlock defines unlocking with Smart Lock
	SmartLockUnlock SubFeature = iota
	// SmartLockLogin defines logging in with Smart Lock
	SmartLockLogin
)
