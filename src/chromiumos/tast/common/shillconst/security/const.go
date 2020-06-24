// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package security defines the constants of shill's security types.
package security

// Security options defined in dbus-constants.h
const (
	WPA       = "wpa"
	WEP       = "wep"
	RSN       = "rsn"
	IEEE8021x = "802_1x"
	PSK       = "psk"
	None      = "none"
)
