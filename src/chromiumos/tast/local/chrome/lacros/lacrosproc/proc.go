// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package lacrosproc provides utilities to find lacros Chrome processes.
package lacrosproc

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/shirou/gopsutil/v3/process"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/internal/chromeproc"
	"chromiumos/tast/local/procutil"
	"chromiumos/tast/testing"
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

// ProcsFromPath returns the pids of all processes with a given path in their
// command line. This is typically used to find all chrome-related binaries,
// e.g. chrome, nacl_helper, etc. They typically share a path, even though their
// binary names differ.
// There may be a race condition between calling this method and using the pids
// later. It's possible that one of the processes is killed, and possibly even
// replaced with a process with the same pid.
func ProcsFromPath(ctx context.Context, path string) ([]*process.Process, error) {
	procs, err := procutil.FindAll(func(p *process.Process) bool {
		exe, err := p.Exe()
		return err == nil && strings.Contains(exe, path)
	})
	if err != nil && !errors.Is(err, procutil.ErrNotFound) {
		return nil, err
	}

	testing.ContextLog(ctx, procs)

	return procs, nil
}
