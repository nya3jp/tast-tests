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
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// DUTCommandRunnerRemote implements libhwsec.DUTCommandRunner in remote test.
type DUTCommandRunnerRemote struct {
	d *dut.DUT
}

// Run implements the one of libhwsec.DUTCommandRunner.
func (r *DUTCommandRunnerRemote) Run(ctx context.Context, cmd string, args ...string) ([]byte, error) {
	testing.ContextLog(ctx, "Running ", cmd, " with arguments: ", args)
	return r.d.Command(cmd, args...).Output(ctx)
}

// NewDUTCommandRunnerRemote creates a new DUTCommandRunnerRemote instance associated with |d|.
func NewDUTCommandRunnerRemote(d *dut.DUT) (libhwsec.DUTCommandRunner, error) {
	if d == nil {
		return nil, errors.New("bad DUT instance")
	}
	return &DUTCommandRunnerRemote{d}, nil
}

// DUTRebooterRemote implements libhwsec.DUTRebooter in remote test.
type DUTRebooterRemote struct {
	d *dut.DUT
}

// Reboot implements the one of libhwsec.DUTRebooter.
func (r *DUTRebooterRemote) Reboot(ctx context.Context) error {
	//	if err := flushCoverageData(ctx, s); err != nil {
	//		testing.ContextLog(ctx, "Failed to flush coverage data")
	//	}
	return r.d.Reboot(ctx)
}

// NewDUTRebooterRemote creates a new DUTRebooterRemote instance associated with |d|.
func NewDUTRebooterRemote(d *dut.DUT) (libhwsec.DUTRebooter, error) {
	if d == nil {
		return nil, errors.New("bad DUT instance")
	}
	return &DUTRebooterRemote{d}, nil
}

// NewHelperRemote creates a new libhwsec.Helper instance that make use of the functions
// implemented by DUTCommandRunnerRemote and NewDUTRebooterRemote
func NewHelperRemote(d *dut.DUT) (*libhwsec.Helper, error) {
	runner, err := NewDUTCommandRunnerRemote(d)
	if err != nil {
		return nil, errors.Wrap(err, "error creating command runner")
	}
	rebooter, err := NewDUTRebooterRemote(d)
	if err != nil {
		return nil, errors.Wrap(err, "error creating rebooter")
	}
	return &libhwsec.Helper{runner, rebooter}, nil
}
