// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"io/ioutil"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/local/camera/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIDocumentScanning,
		Desc:         "Verifies that the document",
		Contacts:     []string{"wtlee@chromium.org", "chromeos-camera-eng@google.com"},
		SoftwareDeps: []string{"camera_app", "chrome", caps.BuiltinOrVividCamera},
		Data:         []string{"cca_ui.js", "document_1280x960.y4m"},
	})
}

type reviewChoice string

const (
	pdf    reviewChoice = "pdf"
	photo               = "photo"
	retake              = "retake"
)

// CCAUIDocumentScanning is the entry point for local document scanning test.
// Since the document conrers detection on preview happens on CrOS VCD, we
// cannot use File VCD to test it. Therefore, we will leave that part to a
// remote test and test it via CameraBox.
func CCAUIDocumentScanning(ctx context.Context, s *testing.State) {
	if err := runTakeDocumentPhoto(ctx, s.DataPath("document_1280x960.y4m"), []string{s.DataPath("cca_ui.js")}, s.OutDir(), pdf); err != nil {
		s.Error("Failed to take document photo and choosing pdf output: ", err)
	}
	if err := runTakeDocumentPhoto(ctx, s.DataPath("document_1280x960.y4m"), []string{s.DataPath("cca_ui.js")}, s.OutDir(), photo); err != nil {
		s.Error("Failed to take document photo and choosing photo output: ", err)
	}
	if err := runTakeDocumentPhoto(ctx, s.DataPath("document_1280x960.y4m"), []string{s.DataPath("cca_ui.js")}, s.OutDir(), retake); err != nil {
		s.Error("Failed to take document photo and retake: ", err)
	}
}

func runTakeDocumentPhoto(ctx context.Context, videoPath string, scriptPaths []string, outDir string, reviewChoice reviewChoice) (retErr error) {
	cr, err := chrome.New(ctx,
		chrome.ExtraArgs(
			"--use-fake-device-for-media-stream",
			"--use-file-for-fake-video-capture="+videoPath))
	if err != nil {
		return errors.Wrap(err, "failed to launch Chrome")
	}
	defer cr.Close(ctx)

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

	// Enable scanner mode in expert mode.
	if err := app.EnableDocumentMode(ctx); err != nil {
		return errors.Wrap(err, "failed to enable scanner mode")
	}

	// Switch to scanner mode.
	if err := app.SwitchMode(ctx, cca.Scanner); err != nil {
		return errors.Wrap(err, "failed to switch to scanner mode")
	}

	// Click shutter button.
	if err := app.ClickShutter(ctx); err != nil {
		return errors.Wrap(err, "failed to click the shutter button")
	}

	// In review mode. Click the button according to the output type.
	if err := app.WaitForVisibleState(ctx, cca.DocumentReviewView, true); err != nil {
		return errors.Wrap(err, "failed to wait for review UI show up")
	}
	var button cca.UIComponent
	switch reviewChoice {
	case pdf:
		button = cca.SaveAsPDFButton
	case photo:
		button = cca.SaveAsPhotoButton
	case retake:
		button = cca.RetakeButton
	}
	if err := app.WaitForVisibleState(ctx, button, true); err != nil {
		return errors.Wrap(err, "failed to wait for review button show up")
	}
	start := time.Now()
	if err := app.Click(ctx, button); err != nil {
		return errors.Wrap(err, "failed to click the review button")
	}

	// Ensure that the result is successfully saved.
	dirs, err := app.SavedDirs(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get CCA default saved path")
	}
	switch reviewChoice {
	case pdf:
		if _, err := app.WaitForFileSaved(ctx, dirs, cca.DocumentPDFPattern, start); err != nil {
			return errors.Wrap(err, "failed to wait for document PDF file")
		}
	case photo:
		if _, err := app.WaitForFileSaved(ctx, dirs, cca.DocumentPhotoPattern, start); err != nil {
			return errors.Wrap(err, "failed to wait for document photo file")
		}
	case retake:
		foundFiles := false
		for _, dir := range dirs {
			files, err := ioutil.ReadDir(dir)
			if err != nil {
				continue
			}
			foundFiles = foundFiles || len(files) > 0
		}
		if foundFiles {
			return errors.New("File is saved unexpectedly when clicking retake button")
		}
	}
	return nil
}
