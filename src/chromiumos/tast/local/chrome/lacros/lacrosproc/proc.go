// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package lacrosproc provides utilities to find lacros Chrome processes.
package lacrosproc

import (
	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/local/chrome/internal/chromeproc"
)

// ExecPath contains the path to the Chrome executable.
const ExecPath = "/run/lacros/chrome"

// Root returns the Process instance of the root lacros-chrome process.
func Root() (*process.Process, error) {
	return chromeproc.Root(ExecPath)
}
