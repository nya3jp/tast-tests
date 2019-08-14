// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package bundlemain provides a main function implementation for a bundle
// to share it from various local bundle executables.
// The most of the frame implementation is in chromiumos/tast/bundle package,
// but some utilities, which lives in support libraries for maintenance,
// need to be injected.
package bundlemain

import (
	"os"

	"chromiumos/tast/bundle"
	"chromiumos/tast/local/faillog"
	"chromiumos/tast/local/ready"
)

// Main is an entry point function for bundles.
func Main() {
	os.Exit(bundle.Local(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, bundle.LocalDelegate{
		Ready:   ready.Wait,
		Faillog: faillog.Save,
	}))
}
