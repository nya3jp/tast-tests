// Copyright 2022 The ChromiumOS Authors
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
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/internal/chromeproc"
	"chromiumos/tast/local/chrome/lacros/lacrosinfo"
	"chromiumos/tast/local/procutil"
	"chromiumos/tast/testing"
)

// lacrosInfo returns a snapshot lacros-chrome info or an error on failure.
func lacrosInfo(ctx context.Context, tconn *chrome.TestConn) (*lacrosinfo.Info, error) {
	info, err := lacrosinfo.Snapshot(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to retrieve lacrosinfo")
	}
	if len(info.LacrosPath) == 0 {
		return nil, errors.Wrap(err, "lacros is not running (received empty LacrosPath)")
	}
	return info, nil
}

// Root returns the Process instance of the root lacros-chrome process.
// tconn is a connection to ash-chrome. If no process can be found, an error is
// returned.
func Root(ctx context.Context, tconn *chrome.TestConn) (*process.Process, error) {
	info, err := lacrosInfo(ctx, tconn)
	if err != nil {
		return nil, err
	}
	return chromeproc.Root(filepath.Join(info.LacrosPath, "chrome"))
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

// RendererProcesses returns lacros-chrome renderer processes. See also
// chromiumos/tast/local/chrome/chromeproc's RendererProcesses(), for ash-chrome.
func RendererProcesses(ctx context.Context, tconn *chrome.TestConn) ([]*process.Process, error) {
	info, err := lacrosInfo(ctx, tconn)
	if err != nil {
		return nil, err
	}
	return chromeproc.RendererProcesses(filepath.Join(info.LacrosPath, "chrome"))
}

// GPUProcesses returns lacros-chrome GPU processes. See also
// chromiumos/tast/local/chrome/chromeproc's GPUProcesses(), for ash-chrome.
func GPUProcesses(ctx context.Context, tconn *chrome.TestConn) ([]*process.Process, error) {
	info, err := lacrosInfo(ctx, tconn)
	if err != nil {
		return nil, err
	}
	return chromeproc.GPUProcesses(filepath.Join(info.LacrosPath, "chrome"))
}
