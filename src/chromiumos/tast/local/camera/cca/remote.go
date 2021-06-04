// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cca

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/local/camera/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// RunPreviewDocumentCornersDetection tests that the detected document corners will be shown while under document scanner mode.
func RunPreviewDocumentCornersDetection(ctx context.Context, scriptPaths []string, outDir string, facing cca.Facing) (retErr error) {
	cr, err := chrome.New(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create Chrome")
	}

	if err := cca.ClearSavedDirs(ctx, cr); err != nil {
		return errors.Wrap(err, "failed to clear saved directory")
	}

	tb, err := testutil.NewTestBridge(ctx, cr, testutil.UseFakeCamera)
	if err != nil {
		return errors.Wrap(err, "failed to construct test bridge")
	}
	defer tb.TearDown(ctx)

	app, err := cca.New(ctx, cr, scriptPaths, outDir, tb)
	if err != nil {
		return errors.Wrap(err, "failed to open CCA")
	}
	defer func(ctx context.Context) {
		if err := app.Close(ctx); err != nil {
			if retErr == nil {
				retErr = errors.Wrap(err, "failed to close app")
			}
		}
	}(ctx)

	if curFacing, err := app.GetFacing(ctx); err != nil {
		return errors.Wrap(err, "failed to get facing")
	} else if curFacing != facing {
		if err := app.SwitchCamera(ctx); err != nil {
			return errors.Wrap(err, "failed to switch camera")
		}
		if err := app.CheckFacing(ctx, facing); err != nil {
			return errors.Wrap(err, "failed to switch to back camera")
		}
	}

	// Enable scanner mode in expert mode.
	if err := app.EnableDocumentMode(ctx); err != nil {
		return errors.Wrap(err, "failed to enable scanner mode")
	}

	// Switch to scanner mode.
	if err := app.SwitchMode(ctx, cca.Scanner); err != nil {
		return errors.Wrap(err, "failed to switch to scanner mode")
	}

	// Verify that document corners are shown in the preview.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		childCount, err := app.ChildCount(ctx, cca.DocumentScannerOverlay)
		if err != nil {
			return errors.Wrap(err, "failed to get child count of the document scanner overlay")
		}
		// When a document is detected, there will be 4 corners and 4 sides appended to the overlay.
		if childCount != 8 {
			return errors.Wrap(err, "no rectangle is found")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return err
	}

	return nil
}
