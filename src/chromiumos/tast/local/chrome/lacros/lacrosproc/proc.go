// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package lacrosproc provides utilities to find lacros Chrome processes.
package lacrosproc

import (
	"path/filepath"

	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/internal/chromeproc"
)

// LacrosLocation specifies how lacros has been deployed,
// since this effects the location of the binary.
type LacrosLocation int

const (
	// Rootfs lacros location.
	Rootfs LacrosLocation = iota
	// Stateful lacros location.
	Stateful
	// Deployed lacros location (e.g. via deploy_chrome.py).
	Deployed
)

const (
	rootfsLacrosExecPath   = "/run/lacros/chrome"
	statefulLacrosExecPath = "/run/imageloader/lacros-*/*/chrome"
	deployedLacrosExecPath = "/usr/local/lacros-chrome"
)

// Root returns the Process instance of the root lacros-chrome process.
// For LacrosLocation Stateful, an error will be returned if multiple
// executables have been found.
func Root(t LacrosLocation) (*process.Process, error) {
	switch t {
	case Rootfs:
		return chromeproc.Root(rootfsLacrosExecPath)
	case Stateful:
		matches, err := filepath.Glob(statefulLacrosExecPath)
		if err != nil {
			return nil, err
		}

		if len(matches) != 1 {
			return nil, errors.Errorf("found %d lacros executables, expected 1. Pattern: %s", len(matches), statefulLacrosExecPath)
		}

		return chromeproc.Root(matches[0])
	case Deployed:
		return chromeproc.Root(deployedLacrosExecPath)
	}
	return nil, errors.Errorf("unknown lacros type %d", t)
}
