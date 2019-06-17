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

// CmdRunnerLocal implements CmdRunner for local test.
type CmdRunnerLocal struct {
}

// NewCmdRunner creates a new command runnder for local test.
func NewCmdRunner() (*CmdRunnerLocal, error) {
	return &CmdRunnerLocal{}, nil
}

// Run implements the one of hwsec.CmdRunner.
func (r *CmdRunnerLocal) Run(ctx context.Context, cmd string, args ...string) ([]byte, error) {
	testing.ContextLogf(ctx, "Running: %s", shutil.EscapeSlice(append([]string{cmd}, args...)))
	return testexec.CommandContext(ctx, cmd, args...).Output()
}

// HelperLocal extends the function set of hwsec.Helper; thoguh, for now we don't have any that kind of function,
type HelperLocal struct {
	hwsec.Helper
}

// NewHelper creates a new hwsec.Helper instance that make use of the functions
// implemented by CmdRunnerLocal.
func NewHelper(ti hwsec.TPMInitializer) (*HelperLocal, error) {
	return &HelperLocal{*hwsec.NewHelper(ti)}, nil
}
