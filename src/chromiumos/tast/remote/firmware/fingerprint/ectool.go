// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fingerprint

import (
	"context"
	"strconv"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/shutil"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
)

// RollbackState is the state of the anti-rollback block.
type RollbackState struct {
	BlockID    int
	MinVersion int
	RWVersion  int
}

// UnmarshalerEctool unmarshals part of ectool's output into a RollbackState.
func (r *RollbackState) UnmarshalerEctool(data []byte) error {
	rollbackInfoMap := parseColonDelimitedOutput(string(data))

	var state RollbackState
	blockID, err := strconv.Atoi(rollbackInfoMap["Rollback block id"])
	if err != nil {
		return errors.Wrap(err, "failed to convert rollback block id")
	}
	state.BlockID = blockID

	minVersion, err := strconv.Atoi(rollbackInfoMap["Rollback min version"])
	if err != nil {
		return errors.Wrap(err, "failed to convert rollback min version")
	}
	state.MinVersion = minVersion

	rwVersion, err := strconv.Atoi(rollbackInfoMap["RW rollback version"])
	if err != nil {
		return errors.Wrap(err, "failed to convert RW rollback version")
	}
	state.RWVersion = rwVersion

	*r = state
	return nil
}

// IsEntropySet checks that entropy has already been set based on the block ID.
//
// If the block ID is greater than 0, there is a very good chance that entropy
// has been added. This is the same way that biod/bio_wash checks if entropy has
// been set. That being said, this method can be fooled if some test simply
// increments the anti-rollback version from a fresh flashing.
func (r *RollbackState) IsEntropySet() bool {
	return r.BlockID > 0
}

// IsAntiRollbackSet checks if version anti-rollback has been enabled.
//
// We currently do not have a minimum version number, thus this function
// indicates if we are not in the normal rollback state.
func (r *RollbackState) IsAntiRollbackSet() bool {
	return r.MinVersion != 0 || r.RWVersion != 0
}

// RollbackInfo returns the rollbackinfo of the fingerprint MCU.
func RollbackInfo(ctx context.Context, d *dut.DUT) (RollbackState, error) {
	cmd := []string{"ectool", "--name=cros_fp", "rollbackinfo"}
	testing.ContextLogf(ctx, "Running command: %s", shutil.EscapeSlice(cmd))
	out, err := d.Conn().Command(cmd[0], cmd[1:]...).Output(ctx, ssh.DumpLogOnError)
	if err != nil {
		return RollbackState{}, errors.Wrap(err, "failed to query FPMCU rollbackinfo")
	}

	var state RollbackState
	err = state.UnmarshalerEctool(out)
	return state, err
}
