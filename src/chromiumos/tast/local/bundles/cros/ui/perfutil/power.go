// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package perfutil

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/upstart"
)

// EnsureDisplayOn ensures powerd is up running and display is on so that ui
// performance is recorded correctly.
func EnsureDisplayOn(ctx context.Context) error {
	if err := upstart.EnsureJobRunning(ctx, "powerd"); err != nil {
		return errors.Wrap(err, "failed to ensure powerd running")
	}

	if err := power.TurnOnDisplay(ctx); err != nil {
		return errors.Wrap(err, "failed to turn on display")
	}

	return nil
}
