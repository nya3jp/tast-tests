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
	"chromiumos/tast/testing"
)

// RunPreviewDocumentCornersDetection tests that the detected document corners will be shown while under document scan mode.
func RunPreviewDocumentCornersDetection(ctx context.Context, scriptPaths []string, outDir string, facing Facing) (retErr error) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

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

	// For the devices with document mode enabled by default, the scan mode button should be visible
	// upon launching the app.
	if visible, err := app.Visible(ctx, ScanModeButton); err != nil {
		return errors.Wrap(err, "failed to check visibility of scan mode button")
	} else if !visible {
		if err := app.EnableDocumentMode(ctx); err != nil {
			return errors.Wrap(err, "failed to enable scan mode")
		}
	}

	// Switch to scan mode.
	if err := app.SwitchMode(ctx, Scan); err != nil {
		return errors.Wrap(err, "failed to switch to scan mode")
	}

	// Verify that document corners are shown in the preview.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		result, err := app.HasClass(ctx, DocumentScannerOverlay, "show-corner-indicator")
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to check class of the document scan overlay"))
		} else if !result {
			return errors.Wrap(err, "no document is found")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return err
	}

	return nil
}
