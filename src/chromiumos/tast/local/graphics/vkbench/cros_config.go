// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vkbench

import (
	"context"
	"path/filepath"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/shutil"
)

const (
	vkbenchRuntimePath = "/usr/local/graphics/vkbench/runtime"
	vkbenchPath        = "/usr/local/graphics/vkbench/vkbench"
)

// CrosConfig is the config to run in ChromeOS.
type CrosConfig struct {
	Hasty bool // If hasty is true, glbench will run in hasty mode.
}

// IsHasty returns true if the given run should run in hasty mode.
func (config *CrosConfig) IsHasty() bool {
	return config.Hasty == true
}

// SetUp initializes the environment to run vkbench in ChromeOS.
func (config *CrosConfig) SetUp(ctx context.Context) error {
	// If UI is running, we must stop it and restore later.
	if err := upstart.StopJob(ctx, "ui"); err != nil {
		return errors.Wrap(err, "failed to stop ui")
	}
	return nil
}

// Run runs vkbench and returns the output.
func (config *CrosConfig) Run(ctx context.Context, fixtValue interface{}, outDir string) (string, error) {
	args := []string{"--runtime_dir", vkbenchRuntimePath, "--out_dir", filepath.Join(outDir, "vkbench")}
	if config.IsHasty() {
		args = append(args, "--hasty")
	}
	cmd := testexec.CommandContext(ctx, vkbenchPath, args...)
	b, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		return "", errors.Wrapf(err, "failed to run vkbench %s", shutil.EscapeSlice(cmd.Args))
	}
	return string(b), nil
}

// TearDown tears down the environment.
func (config *CrosConfig) TearDown(ctx context.Context) error {
	if err := upstart.EnsureJobRunning(ctx, "ui"); err != nil {
		return errors.Wrap(err, "failed to start ui")
	}
	return nil
}
