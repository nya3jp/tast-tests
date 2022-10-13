// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package wmp contains utility functions for window management and performance.
package wmp

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

// RecordWindowScreenCapture records an open window using Screen Capture in Quick Settings.
func RecordWindowScreenCapture(ctx context.Context, tconn *chrome.TestConn, downloadsPath string) error {
	ui := uiauto.New(tconn)

	activeWindow, err := ash.GetActiveWindow(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to find active window")
	}
	centerPoint := activeWindow.BoundsInRoot.CenterPoint()

	if err = uiauto.Combine("start recording an open window",
		ui.LeftClick(nodewith.Role(role.ToggleButton).Name("Screen record")),
		ui.LeftClick(nodewith.Role(role.ToggleButton).Name("Record window")),
		mouse.Move(tconn, centerPoint, 200*time.Millisecond),
		ui.LeftClick(nodewith.Role(role.Window).First()),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to start recording an open window")
	}

	testing.Sleep(ctx, 10*time.Second)

	if err := ui.LeftClick(nodewith.Name("Stop screen recording").First())(ctx); err != nil {
		return errors.Wrap(err, "failed to stop recording an open window")
	}

	filename := filepath.Join(downloadsPath, "Screen recording*.webm")
	if _, err := os.Stat(filename); err != nil {
		errors.Wrap(err, "failed to file the recording file")
	}

	return nil
}

// RecordWindowScreenShare records an open window using screen share.
func RecordWindowScreenShare(ctx context.Context, tconn *chrome.TestConn, downloadsPath string) error {
	recorder, err := uiauto.NewWindowRecorder(ctx, tconn, 0)
	if err != nil {
		errors.Wrap(err, "failed to create screen recorder")
	}

	if err := recorder.Start(ctx, tconn); err != nil {
		errors.Wrap(err, "failed to start screen recorder")
	}
	testing.Sleep(ctx, 10*time.Second)

	if err := recorder.Stop(ctx); err != nil {
		errors.Wrap(err, "failed to stop screen recorder")
	}

	filename := filepath.Join(downloadsPath, "Screen recording share.webm")
	if err := recorder.SaveInBytes(ctx, filename); err != nil {
		errors.Wrap(err, "failed to save screen recorder")
	}

	if _, err := os.Stat(filename); err != nil {
		errors.Wrap(err, "failed to file the recording file")
	}

	return nil
}

// DeleteAllRecordings deletes all screen recording files located in the user downloads folder.
func DeleteAllRecordings(downloadsPath string) error {
	const recordingPattern = "Screen recording*.webm"
	files, err := filepath.Glob(filepath.Join(downloadsPath, recordingPattern))
	if err != nil {
		return errors.Wrapf(err, "the pattern %q is malformed", recordingPattern)
	}

	for _, f := range files {
		if err := os.Remove(f); err != nil {
			return errors.Wrap(err, "failed to delete the recording")
		}
	}

	return nil
}
