// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package glbench

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/shutil"
)

// CrostiniConfig is the config to run in Crostini.
type CrostiniConfig struct {
	Hasty bool // If hasty is true, glbench will run in hasty mode.
}

// IsHasty returns true if the given run should run in hasty mode.
func (config *CrostiniConfig) IsHasty() bool {
	return config.Hasty == true
}

// SetUp initialized the environement to run glbench in Crostini.
func (config *CrostiniConfig) SetUp(ctx context.Context) error {
	// Disable the display to avoid vsync.
	if err := power.SetDisplayPower(ctx, power.DisplayPowerAllOff); err != nil {
		return errors.Wrap(err, "failed to power off display")
	}
	return nil
}

// Run runs glbench and returns the output.
func (config *CrostiniConfig) Run(ctx context.Context, preValue interface{}, outDir string) (string, error) {
	cont := preValue.(crostini.PreData).Container
	if err := cont.Command(ctx, "dpkg", "-s", "glbench").Run(); err != nil {
		return "", errors.Wrap(err, "glbench is not installed")
	}
	args := []string{"-outdir=glbench_results", "-save", "-notemp"}
	if config.IsHasty() {
		args = append(args, "-hasty")
	}

	// In crostini, glbench is preinstalled in PATH.
	cmd := cont.Command(ctx, append([]string{"glbench"}, args...)...)
	b, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		return "", errors.Wrapf(err, "failed to run %s", shutil.EscapeSlice(cmd.Args))
	}

	// Move result file out of the crostini container.
	if err := cont.GetFile(ctx, "glbench_results", outDir); err != nil {
		return "", errors.Wrap(err, "failed to get results from container")
	}
	return string(b), nil
}

// TearDown teardown the environement.
func (config *CrostiniConfig) TearDown(ctx context.Context) error {
	if err := power.SetDisplayPower(ctx, power.DisplayPowerAllOff); err != nil {
		return errors.Wrap(err, "failed to power on display")
	}
	return nil
}
