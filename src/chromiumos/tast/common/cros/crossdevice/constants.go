// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package crossdevice is used for Cross Device functionality.
package crossdevice

// CrosAttributes contains information about the CrOS device that are relevant to Nearby Share.
type CrosAttributes struct {
	User            string
	ChromeVersion   string
	ChromeOSVersion string
	Board           string
	Model           string
}
