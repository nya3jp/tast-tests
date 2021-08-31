// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cca

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/camera/testutil"
	"chromiumos/tast/local/chrome"
)

// RunPortraitModeTesting tests that portrait mode works expectedly.
func RunPortraitModeTesting(ctx context.Context, scriptPaths []string, outDir string, facing Facing) (retErr error) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// TODO(b/177800595): To avoid creating Chrome for every CCARemote tests, we
	// may need to refactor them to local tests and using fixtures just like
	// other CCA local tests did.
	cr, err := chrome.New(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create Chrome")
	}
	defer cr.Close(cleanupCtx)

	if err := ClearSavedDir(ctx, cr); err != nil {
		return errors.Wrap(err, "failed to clear saved directory")
	}

	tb, err := testutil.NewTestBridge(ctx, cr, testutil.UseFakeCamera)
	if err != nil {
		return errors.Wrap(err, "failed to construct test bridge")
	}
	defer tb.TearDown(cleanupCtx)

	app, err := New(ctx, cr, scriptPaths, outDir, tb)
	if err != nil {
		return errors.Wrap(err, "failed to open CCA")
	}
	defer func(cleanupCtx context.Context) {
		if err := app.Close(cleanupCtx); err != nil {
			if retErr == nil {
				retErr = errors.Wrap(err, "failed to close app")
			}
		}
	}(cleanupCtx)

	if curFacing, err := app.GetFacing(ctx); err != nil {
		return errors.Wrap(err, "failed to get facing")
	} else if curFacing != facing {
		if err := app.SwitchCamera(ctx); err != nil {
			return errors.Wrap(err, "failed to switch camera")
		}
		if err := app.CheckFacing(ctx, facing); err != nil {
			return errors.Wrapf(err, "failed to switch to the target camera: %v", facing)
		}
	}

	if supported, err := app.PortraitModeSupported(ctx); err != nil {
		return errors.Wrap(err, "failed to determine whether portrait mode is supported")
	} else if supported {
		if err := app.SwitchMode(ctx, Portrait); err != nil {
			return errors.Wrap(err, "failed to switch to portrait mode")
		}
		if _, err = app.TakeSinglePhoto(ctx, TimerOff); err != nil {
			return errors.Wrap(err, "failed to take portrait photo")
		}
	}
	return nil
}
