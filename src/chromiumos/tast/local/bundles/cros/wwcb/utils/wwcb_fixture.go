// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package utils

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// InitFixtures turns on dock power at the beginning, and returns
// a function that restores the fixtures status setting.
// To avoid peripherals status were cached on dock and cause testing inaccurate.
func InitFixtures(ctx context.Context) (func(context.Context) error, error) {
	testing.ContextLog(ctx, "Initialize fixtures")

	if err := ControlAviosys(ctx, "1", "1"); err != nil {
		return nil, errors.Wrap(err, "failed to turn on docking station power")
	}

	return func(ctx context.Context) error {
		testing.ContextLog(ctx, "Disconnecting all fixtures and closing dock power")
		// Disconnect all fixtures.
		if err := CloseAll(ctx); err != nil {
			return errors.Wrap(err, "failed to disconnect all fixtures")
		}
		// Turn off docking station power.
		if err := ControlAviosys(ctx, "0", "1"); err != nil {
			return errors.Wrap(err, "failed to turn off docking station power")
		}
		return nil
	}, nil
}

// DumpScreenshotOnError checks the given hasError function and dumps each display screenshot in WWCB server.
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
