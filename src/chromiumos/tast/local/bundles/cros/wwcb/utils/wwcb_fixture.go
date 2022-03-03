// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package utils

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// InitFixtures resets all fixtures at the beginning of testing.
// To avoid fixtures status are not unified then cause fail.
func InitFixtures(ctx context.Context) error {
	testing.ContextLog(ctx, "Initialize fixtures")
	// Disconnect all fixtures.
	if err := CloseAll(ctx); err != nil {
		return err
	}
	// Docking station uses port 1.
	// Turn off & on docking station power.
	if err := ControlAviosys(ctx, "0", "1"); err != nil {
		return errors.Wrap(err, "failed to turn off docking station power")
	}
	if err := ControlAviosys(ctx, "1", "1"); err != nil {
		return errors.Wrap(err, "failed to turn on docking station power")
	}
	return nil
}
