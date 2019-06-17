// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

/*
This file implements miscellaneous and unsorted helpers.
*/

import (
	"context"

	libhwsec "chromiumos/tast/common/hwsec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

// CmdRunnerLocal implements libhwsec.CmdRunner in remote test.
type CmdRunnerLocal struct {
}

// Run implements the one of libhwsec.CmdRunner.
func (r *CmdRunnerLocal) Run(ctx context.Context, cmd string, args ...string) ([]byte, error) {
	testing.ContextLogf(ctx, "Running: %s", shutil.EscapeSlice(append([]string{cmd}, args...)))
	return testexec.CommandContext(ctx, cmd, args...).Output()
}

// DUTRebooterLocal implements libhwsec.DUTRebooter in remote test.
type DUTRebooterLocal struct {
}

// Reboot implements the one of libhwsec.DUTRebooter.
func (r *DUTRebooterLocal) Reboot(ctx context.Context) error {
	return errors.New("Not implemented")
}

// NewDUTRebooterLocal creates a new DUTRebooterLocal instance associated with |d|.
func NewDUTRebooterLocal() (libhwsec.DUTRebooter, error) {
	return &DUTRebooterLocal{}, nil
}

// NewHelperLocal creates a new libhwsec.Helper instance that make use of the functions
// implemented by CmdRunnerLocal and NewDUTRebooterLocal
func NewHelperLocal() (*libhwsec.Helper, error) {
	runner := &CmdRunnerLocal{}
	rebooter, err := NewDUTRebooterLocal()
	if err != nil {
		return nil, errors.Wrap(err, "error creating rebooter")
	}
	return &libhwsec.Helper{runner, rebooter}, nil
}
