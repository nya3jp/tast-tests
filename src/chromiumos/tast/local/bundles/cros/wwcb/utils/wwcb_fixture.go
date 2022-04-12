// Copyright 2022 The ChromiumOS Authors.
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
	// Disconnect all fixtures.
	if err := CloseAll(ctx); err != nil {
		return err
	}
	// Turn off & on docking station power.
	if err := ControlAviosys(ctx, "0", "1"); err != nil {
		return errors.Wrap(err, "failed to turn off docking station power")
	}
	if err := ControlAviosys(ctx, "1", "1"); err != nil {
		return errors.Wrap(err, "failed to turn on docking station power")
	}
	return nil
}

// DumpScreenshotOnError checks the given hasError function and dumps each display screenshot in the WWCB server.
func DumpScreenshotOnError(ctx context.Context, hasError func() bool, fixtureIDs []string) {
	if !hasError() {
		return
	}

	testing.ContextLog(ctx, "Start dumping screenshot")
	fixtureIDs = append(fixtureIDs, "chromebook")
	for _, id := range fixtureIDs {
		if _, err := ScreenCapture(ctx, id); err != nil {
			testing.ContextLog(ctx, "Failed to capture screenshot: ", err)
		}
	}
}
