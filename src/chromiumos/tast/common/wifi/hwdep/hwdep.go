// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package hwdep provides hardware dependencies for some WiFi standards.
package hwdep

import "chromiumos/tast/testing/hwdep"

// Require80211ac skips Monroe (ath9k) since it does not support 802.11ac.
func Require80211ac() hwdep.Condition {
	return hwdep.SkipOnPlatform("monroe")
}
