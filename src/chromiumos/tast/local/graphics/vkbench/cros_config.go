// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vkbench

import (
	"context"
	"path/filepath"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/shutil"
)

const (
	vkbenchShaderPath = "/usr/local/graphics/vkbench/shaders"
	vkbenchPath       = "/usr/local/graphics/vkbench/vkbench"
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
	return nil
}

// Run runs vkbench and returns the output.
func (config *CrosConfig) Run(ctx context.Context, fixtValue interface{}, outDir string) (string, error) {
	args := []string{"--spirv_dir", vkbenchShaderPath, "--out_dir", filepath.Join(outDir, "vkbench")}
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
	return nil
}
