// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package main implements the "mtbf" local test bundle.
//
// This executable contains standard Chrome OS tests.
package main

import (
	"chromiumos/tast/local/bundlemain"
	// Underscore-imported packages register their tests via init functions.
	_ "chromiumos/tast/local/bundles/mtbf/audio"
	_ "chromiumos/tast/local/bundles/mtbf/camera"
	_ "chromiumos/tast/local/bundles/mtbf/video"
)

func main() {
	bundlemain.Main()
}
