// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package main implements the "cros" local test bundle.
//
// This executable contains standard ChromeOS tests.
package main

import (
	"chromiumos/tast/local/bundlemain"
)

func main() {
	bundlemain.RunLocal()
}
