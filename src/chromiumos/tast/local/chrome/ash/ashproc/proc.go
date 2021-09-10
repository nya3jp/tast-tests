// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ashproc provides utilities to find ash Chrome (a.k.a. chromeos-chrome) processes.
package ashproc

import (
	"context"
	"time"

	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/internal/chromeproc"
	"chromiumos/tast/testing"
)

// ExecPath contains the path to the Chrome executable.
const ExecPath = "/opt/google/chrome/chrome"

// Processes returns ash-chrome processes, including crashpad_handler processes, too.
func Processes() ([]*process.Process, error) {
	return chromeproc.Processes(ExecPath)
}

// Root returns the Process instance of the root ash-chrome process.
func Root() (*process.Process, error) {
	return chromeproc.Root(ExecPath)
}

// WaitForRoot waits for ash-chrome's root process is launched.
func WaitForRoot(ctx context.Context, timeout time.Duration) (*process.Process, error) {
	var ret *process.Process
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var err error
		ret, err = Root()
		return err
	}, &testing.PollOptions{Timeout: timeout}); err != nil {
		return nil, errors.Wrap(err, "waiting for ash-chrome root is timed out")
	}
	return ret, nil
}
