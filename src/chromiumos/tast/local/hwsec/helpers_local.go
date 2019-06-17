// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

/*
This file implements miscellaneous and unsorted helpers.
*/

import (
	"context"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

type cmdRunnerLocal struct {
}

// NewCmdRunner creates a new command runnder for local test.
func NewCmdRunner() (*cmdRunnerLocal, error) {
	return &cmdRunnerLocal{}, nil
}

// Run implements the one of hwsec.CmdRunner.
func (r *cmdRunnerLocal) Run(ctx context.Context, cmd string, args ...string) ([]byte, error) {
	testing.ContextLogf(ctx, "Running: %s", shutil.EscapeSlice(append([]string{cmd}, args...)))
	return testexec.CommandContext(ctx, cmd, args...).Output()
}

// RunShell implements the one of hwsec.CmdRunner.
func (r *cmdRunnerLocal) RunShell(ctx context.Context, cmd string) ([]byte, error) {
	return hwsec.RunShell(ctx, r, cmd)
}

type helperLocal struct {
	hwsec.Helper
}

// NewHelper creates a new hwsec.Helper instance that make use of the functions
// implemented by cmdRunnerLocal.
func NewHelper(ti hwsec.TpmInitializer) (*helperLocal, error) {
	return &helperLocal{*hwsec.NewHelper(ti)}, nil
}
