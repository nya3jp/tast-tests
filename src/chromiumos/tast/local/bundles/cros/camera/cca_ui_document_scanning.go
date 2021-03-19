// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"io/ioutil"
	"os"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIDocumentScanning,
		Desc:         "Verifies that CCA can take a photo for document and generate the document file via file VCD",
		Contacts:     []string{"wtlee@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:camera-libcamera"},
		SoftwareDeps: []string{"camera_app", "chrome", "ondevice_document_scanner", caps.BuiltinOrVividCamera},
		Fixture:      "ccaTestBridgeReadyWithDocumentScene",
	})
}

type reviewChoice string

const (
	pdf    reviewChoice = "pdf"
	photo               = "photo"
	retake              = "retake"
)

// CCAUIDocumentScanning is the entry point for local document scanning test.
// We use File VCD with a video which has a document in the scene to simulate
// the real usage when scanning document.
// However, since document detection on preview only happens on CrOS VCD, we
// cannot use File VCD to test it. Therefore, we will leave that part to a
// remote test and test it via CameraBox.
func CCAUIDocumentScanning(ctx context.Context, s *testing.State) {
	startApp := s.FixtValue().(cca.FixtureData).StartApp
	stopApp := s.FixtValue().(cca.FixtureData).StopApp
	s.FixtValue().(cca.FixtureData).SetDebugParams(cca.DebugParams{SaveCameraFolderWhenFail: true})

	subTestTimeout := 30 * time.Second
	for _, tst := range []struct {
		name    string
		choices []reviewChoice
	}{{
		"testPDF",
		[]reviewChoice{pdf},
	}, {
		"testPhoto",
		[]reviewChoice{photo},
	}, {
		"testRetakeThenTakePhoto",
		[]reviewChoice{retake, photo},
	}} {
		subTestCtx, cancel := context.WithTimeout(ctx, subTestTimeout)
		s.Run(subTestCtx, tst.name, func(ctx context.Context, s *testing.State) {
			app, err := startApp(ctx)
			if err != nil {
				s.Fatal("Failed to start app: ", err)
			}
			cleanupCtx := ctx
			ctx, cancel := ctxutil.Shorten(ctx, 3*time.Second)
			defer cancel()
			defer func(cleanupCtx context.Context) {
				if err := stopApp(cleanupCtx, s.HasError()); err != nil {
					s.Fatal("Failed to stop app: ", err)
				}
			}(cleanupCtx)

			if err := runTakeDocumentPhoto(ctx, app, tst.choices); err != nil {
				s.Fatalf("Failed to take document photos under sub test %v: %v", tst.name, err)
			}
		})
		cancel()
	}
}

// runTakeDocumentPhoto tests if CCA can take a document photo and generate document file correctly.
func runTakeDocumentPhoto(ctx context.Context, app *cca.App, reviewChoices []reviewChoice) (retErr error) {
	// For the devices with document mode enabled by default, the scan mode button should be visible
	// upon launching the app.
	if visible, err := app.Visible(ctx, cca.ScanModeButton); err != nil {
		return errors.Wrap(err, "failed to check visibility of scan mode button")
	} else if !visible {
		if err := app.EnableDocumentMode(ctx); err != nil {
			return errors.Wrap(err, "failed to enable scan mode")
		}
	}

	if err := app.SwitchMode(ctx, cca.Scan); err != nil {
		return errors.Wrap(err, "failed to switch to scan mode")
	}

	if checked, err := app.IsCheckedWithIndex(ctx, cca.ScanDocumentModeOption, 0); err != nil {
		return errors.Wrap(err, "failed to check if it lands on document mode")
	} else if !checked {
		return errors.New("failed to land on document mode by default")
	}

	for _, reviewChoice := range reviewChoices {
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

		if err := app.WaitForState(ctx, "taking", false); err != nil {
			return errors.Wrap(err, "failed to wait for taking state to be false after clicking retake")
		}

		// Ensure that the result is successfully saved.
		dir, err := app.SavedDir(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get CCA default saved path")
		}
		switch reviewChoice {
		case pdf:
			if _, err := app.WaitForFileSaved(ctx, dir, cca.DocumentPDFPattern, start); err != nil {
				return errors.Wrap(err, "failed to wait for document PDF file")
			}
		case photo:
			if _, err := app.WaitForFileSaved(ctx, dir, cca.DocumentPhotoPattern, start); err != nil {
				return errors.Wrap(err, "failed to wait for document photo file")
			}
		case retake:
			// When users click the "Retake" button, the captured data should be
			// dropped and no files should be saved.
			if _, err := os.Stat(dir); err != nil {
				if os.IsNotExist(err) {
					return nil
				}
				return errors.Wrap(err, "failed when check camera folder")
			}

			files, err := ioutil.ReadDir(dir)
			if err != nil {
				return errors.Wrap(err, "failed to read camera folder")
			}

			for _, file := range files {
				if file.ModTime().After(start) {
					return errors.New("file is saved unexpectedly when clicking retake button")
				}
			}
		}
	}
	return nil
}
