// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package main implements the "cros" local test bundle.
//
// This executable contains standard Chrome OS tests.
package main

import (
	"os"

	"chromiumos/tast/bundle"

	// These packages register their tests via init functions.
	_ "chromiumos/tast/local/bundles/cros/example"
	_ "chromiumos/tast/local/bundles/cros/power"
	_ "chromiumos/tast/local/bundles/cros/security"
	_ "chromiumos/tast/local/bundles/cros/ui"
)

func main() {
	os.Exit(bundle.Local(os.Stdin, os.Stdout))
}
