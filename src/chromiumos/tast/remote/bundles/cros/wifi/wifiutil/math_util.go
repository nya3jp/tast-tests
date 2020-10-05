// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifiutil

// Max return the greater value of a, b.
func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Min return the lesser value of a, b.
func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Clamp returns x if it's in range of <xmin, xmax>, otherwise one of the limits if it's below or above.
func Clamp(x, xmin, xmax int) int {
	return Min(xmax, Max(x, xmin))
}
