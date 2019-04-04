// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package main implements the "cros" local test bundle.
//
// This executable contains standard Chrome OS tests.
package main

import (
	"os"

	// Underscore-imported packages register their tests via init functions.
	"chromiumos/tast/bundle"
	_ "chromiumos/tast/local/bundles/cros/arc"
	_ "chromiumos/tast/local/bundles/cros/audio"
	_ "chromiumos/tast/local/bundles/cros/camera"
	_ "chromiumos/tast/local/bundles/cros/cryptohome"
	_ "chromiumos/tast/local/bundles/cros/debugd"
	_ "chromiumos/tast/local/bundles/cros/example"
	_ "chromiumos/tast/local/bundles/cros/graphics"
	_ "chromiumos/tast/local/bundles/cros/hardware"
	_ "chromiumos/tast/local/bundles/cros/kernel"
	_ "chromiumos/tast/local/bundles/cros/meta"
	_ "chromiumos/tast/local/bundles/cros/network"
	_ "chromiumos/tast/local/bundles/cros/platform"
	_ "chromiumos/tast/local/bundles/cros/power"
	_ "chromiumos/tast/local/bundles/cros/printer"
	_ "chromiumos/tast/local/bundles/cros/security"
	_ "chromiumos/tast/local/bundles/cros/session"
	_ "chromiumos/tast/local/bundles/cros/ui"
	_ "chromiumos/tast/local/bundles/cros/video"
	_ "chromiumos/tast/local/bundles/cros/vm"
	"chromiumos/tast/local/ready"
)

func main() {
	os.Exit(bundle.Local(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, ready.Wait))
}
