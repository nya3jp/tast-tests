// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package main implements the "mtbf" remote test bundle.
//
// This executable contains standard Chrome OS tests.
package main

import (
	"os"

	"chromiumos/tast/bundle"

	// These packages register their tests via init functions.
	_ "chromiumos/tast/remote/bundles/mtbf/meta"
)

func main() {
	os.Exit(bundle.Remote(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
