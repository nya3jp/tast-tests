// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policyutil

import (
	"context"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/hwsec"
)

// EnsureTPMIsResetAndPowerwash initialises the required helpers and calls HelperRemote.EnsureTPMIsResetAndPowerwash.
func EnsureTPMIsResetAndPowerwash(ctx context.Context, d *dut.DUT) error {
	r := hwsec.NewCmdRunner(d)

	helper, err := hwsec.NewHelper(r, d)
	if err != nil {
		return errors.Wrap(err, "helper creation error")
	}

	if err := helper.EnsureTPMIsResetAndPowerwash(ctx); err != nil {
		return errors.Wrap(err, "failed to reset system")
	}

	return nil
}
