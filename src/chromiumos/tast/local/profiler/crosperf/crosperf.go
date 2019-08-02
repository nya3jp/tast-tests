// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package crosperf supports capture system profiler data while running test
// in ChromeOS. It offers the support of gathering profiler data using the
// command "perf record".
package crosperf

import (
	"context"
	"path/filepath"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/profiler/controller"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

var cmd *testexec.Cmd

func start(ctx context.Context, s *testing.State) error {
	outputPath := filepath.Join(s.OutDir(), "perf.data")
	cmd := testexec.CommandContext(ctx, "perf", "record", "-e", "cycles", "-g", "--output", outputPath)
	if err := cmd.Start(); err != nil {
		cmd.DumpLog(ctx)
		return errors.Wrapf(err, "failed running %q", strings.Join(cmd.Args, " "))
	}
	return nil
}

func end() error {
	var err error
	if cmd != nil {
		err = cmd.Kill()
		if err == nil {
			err = cmd.Wait()
		}
		if err != nil {
			return errors.Wrap(err, "failed ending")
		}
	}
	return nil
}

// Register added crosperf profiler to the controller.
func Register() {
	controller.RegisterProfiler("cros_perf", start, end)
}
