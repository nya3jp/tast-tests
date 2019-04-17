// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package main implements the "cros" remote test bundle.
//
// This executable contains standard Chrome OS tests.
package main

import (
	"os"

	"chromiumos/tast/bundle"

	// These packages register their tests via init functions.
	_ "chromiumos/tast/remote/bundles/cros/example"
	_ "chromiumos/tast/remote/bundles/cros/firmware"
	_ "chromiumos/tast/remote/bundles/cros/meta"
	_ "chromiumos/tast/remote/bundles/cros/power"
)

func main() {
	os.Exit(bundle.Remote(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
