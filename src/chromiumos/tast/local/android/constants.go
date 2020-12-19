// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package android

// DefaultUser is the default Android user ID, corresponding to the system user that adb runs as.
const DefaultUser = "0"

// DownloadDir is Android's default downloads directory.
const DownloadDir = "/sdcard/Download/"

// KeyCode is used to specify a key event on the Android device using the `adb shell input keyevent` command.
type KeyCode string

// Derived from https://developer.android.com/reference/android/view/KeyEvent.html.
const (
	KeyCodeWakeup KeyCode = "KEYCODE_WAKEUP"
)
