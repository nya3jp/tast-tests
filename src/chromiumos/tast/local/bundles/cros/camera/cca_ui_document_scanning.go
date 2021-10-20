// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"io/ioutil"
	"math"
	"os"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/coords"
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

type docCorner struct {
	x, y float64
}

type docArea struct {
	// corners is coordinates of document corners starts from left-top
	// corner and in counter-clockwise order. Numbers are normalized with
	// width, height of original image(before cropping).
	corners []docCorner
}

// similar compares if two area are similar with tolerance.
func (area *docArea) similar(area2 *docArea) error {
	const tolerance = 0.1
	for i := 0; i < 4; i++ {
		corn1 := area.corners[i]
		corn2 := area2.corners[i]
		if math.Abs(corn1.x-corn2.x) > tolerance {
			return errors.Errorf("coordindate x mismatch for comparing document area %v and %v", area, area2)
		}
		if math.Abs(corn1.y-corn2.y) > tolerance {
			return errors.Errorf("coordindate y mismatch for comparing document area %v and %v", area, area2)
		}
	}
	return nil
}

var (
	// The shorter document on the left of camera scene.
	doc1Area = &docArea{[]docCorner{
		docCorner{0.05298, 0.44720},
		docCorner{0.03376, 0.84603},
		docCorner{0.49621, 0.77297},
		docCorner{0.46445, 0.39194},
	}}
	// The longer document on the right of camera scene.
	doc2Area = &docArea{[]docCorner{
		docCorner{0.53727, 0.16051},
		docCorner{0.56251, 0.88772},
		docCorner{0.99996, 0.86272},
		docCorner{0.89309, 0.15380},
	}}
)

// CCAUIDocumentScanning is the entry point for local document scanning test.
// We use File VCD with a video which has a document in the scene to simulate
// the real usage when scanning document.
// However, since document detection on preview only happens on CrOS VCD, we
// cannot use File VCD to test it. Therefore, we will leave that part to another
// test which is executed on a CameraBox.
func CCAUIDocumentScanning(ctx context.Context, s *testing.State) {
	runSubTest := s.FixtValue().(cca.FixtureData).RunSubTest
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
			if err := runSubTest(ctx, func(ctx context.Context, app *cca.App) error {
				return runTakeDocumentPhoto(ctx, app, tst.choices)
			}, cca.SubTestParams{}); err != nil {
				s.Errorf("Failed to pass %v subtest: %v", tst.name, err)
			}
		})
		cancel()
	}
	subTestCtx, cancel := context.WithTimeout(ctx, subTestTimeout)
	s.Run(subTestCtx, "testFixCropArea", func(ctx context.Context, s *testing.State) {

		if err := runSubTest(ctx, func(ctx context.Context, app *cca.App) error {
			cr := s.FixtValue().(cca.FixtureData).Chrome
			return testFixCropArea(ctx, app, cr)
		}, cca.SubTestParams{}); err != nil {
			s.Error("Failed to pass testFixCropArea subtest: ", err)
		}
	})
	cancel()
}

func enterDocumentMode(ctx context.Context, app *cca.App) error {
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
	return nil
}

// runTakeDocumentPhoto tests if CCA can take a document photo and generate document file correctly.
func runTakeDocumentPhoto(ctx context.Context, app *cca.App, reviewChoices []reviewChoice) (retErr error) {
	if err := enterDocumentMode(ctx, app); err != nil {
		return errors.Wrap(err, "failed to enter document mode")
	}

	for _, reviewChoice := range reviewChoices {
		if err := app.ClickShutter(ctx); err != nil {
			return errors.Wrap(err, "failed to click the shutter button")
		}

		// In review mode. Click the button according to the output type.
		if err := app.WaitForVisibleState(ctx, cca.ReviewView, true); err != nil {
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

func testFixCropArea(ctx context.Context, app *cca.App, cr *chrome.Chrome) error {
	if err := enterDocumentMode(ctx, app); err != nil {
		return errors.Wrap(err, "failed to enter document mode")
	}

	if err := app.ClickShutter(ctx); err != nil {
		return errors.Wrap(err, "failed to click the shutter button")
	}

	if err := app.WaitForVisibleState(ctx, cca.ReviewView, true); err != nil {
		return errors.Wrap(err, "failed to wait for review UI show up")
	}

	reviewElSize, err := app.Size(ctx, cca.ReviewImage)
	if err != nil {
		return errors.Wrap(err, "failed to get review size at initial scan")
	}

	if reviewElSize.Width <= reviewElSize.Height {
		return errors.New("Should crop shorter document at initial scan")
	}

	if err := app.Click(ctx, cca.FixCropButton); err != nil {
		return errors.Wrap(err, "failed to click the fix button")
	}

	if err := app.WaitForVisibleState(ctx, cca.CropDocumentView, true); err != nil {
		return errors.Wrap(err, "failed to wait for review UI show up")
	}

	cropElSize, err := app.Size(ctx, cca.CropDocumentImage)
	if err != nil {
		return err
	}

	cropElPt, err := app.ScreenXYWithIndex(ctx, cca.CropDocumentImage, 0)
	if err != nil {
		return err
	}

	dotElSize, err := app.Size(ctx, cca.DocumentCorner)
	if err != nil {
		return err
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return err
	}

	var corners []docCorner
	for i := 0; i < 4; i++ {
		pt, err := app.ScreenXYWithIndex(ctx, cca.DocumentCorner, i)
		if err != nil {
			return err
		}
		pt.X += dotElSize.Width / 2
		pt.Y += dotElSize.Height / 2
		corners = append(corners, docCorner{
			float64(pt.X-cropElPt.X) / float64(cropElSize.Width),
			float64(pt.Y-cropElPt.Y) / float64(cropElSize.Height)})
	}
	initialDocArea := &docArea{corners}
	if err := doc1Area.similar(initialDocArea); err != nil {
		return errors.Wrap(err, "Mismatch document corner coordinate at initial scan")
	}

	// Drag corners to longer doc on right side. Drag must be done in
	// clockwise order to prevent hitting any checking convex constraint.
	for i := 3; i >= 0; i-- {
		toScreenPt := func(corn docCorner) coords.Point {
			return coords.NewPoint(
				int(corn.x*float64(cropElSize.Width))+cropElPt.X,
				int(corn.y*float64(cropElSize.Height))+cropElPt.Y,
			)
		}
		if err := mouse.Drag(
			tconn, toScreenPt(initialDocArea.corners[i]), toScreenPt(doc2Area.corners[i]),
			300*time.Millisecond)(ctx); err != nil {
			return errors.Wrap(err, "failed to drag corner")
		}
	}
	if err := app.Click(ctx, cca.CropDoneButton); err != nil {
		return errors.Wrap(err, "failed to click the done button")
	}

	// Wait for processing finish and back to review view.
	if err := app.WaitForVisibleState(ctx, cca.CropDocumentView, false); err != nil {
		return errors.Wrap(err, "failed to wait for crop document view closed")
	}
	if err := app.WaitForState(ctx, "view-flash", false); err != nil {
		return errors.Wrap(err, "failed to wait for flash view closed")
	}

	reviewElSize, err = app.Size(ctx, cca.ReviewImage)
	if err != nil {
		return err
	}
	if reviewElSize.Width >= reviewElSize.Height {
		return errors.New("Should crop the longer document after fix crop area")
	}

	return nil
}
