// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"os"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/local/camera/histogramutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         HDRnetE2E,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Runs the HDRnet end-to-end integration test",
		Contacts:     []string{"jcliang@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"camera_app", "camera_feature_hdrnet", "chrome", caps.BuiltinMIPICamera},
		Fixture:      "ccaTestBridgeReady",
		Timeout:      6 * time.Minute,
	})
}

// createFile creates the file specified by |filePath| if it does not exist.
func createFile(filePath string) error {
	file, err := os.OpenFile(filePath, os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		return err
	}
	return file.Close()
}

// HDRnetE2E runs the HDRnet end-to-end subtests.
func HDRnetE2E(ctx context.Context, s *testing.State) {
	hdrnetEnablePath := "/run/camera/force_enable_hdrnet"
	if err := createFile(hdrnetEnablePath); err != nil {
		s.Fatalf("Failed to create HDRnet force enable file %s: %s", hdrnetEnablePath, err)
	}
	defer func() {
		if err := os.Remove(hdrnetEnablePath); err != nil {
			s.Errorf("Failed to remove HDRnet force enable file %s: %s", hdrnetEnablePath, err)
		}
	}()

	runSubTest := s.FixtValue().(cca.FixtureData).RunTestWithApp
	cr := s.FixtValue().(cca.FixtureData).Chrome
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to establish connection to the test API extension")
	}
	subTestTimeout := 120 * time.Second

	for _, t := range []struct {
		name     string
		testFunc func(context.Context, *cca.App, *chrome.TestConn) error
	}{
		{"testPhotoTaking", testPhotoTaking},
		{"testVideoRecording", testVideoRecording},
	} {
		subTestCtx, cancel := context.WithTimeout(ctx, subTestTimeout)
		s.Run(subTestCtx, t.name, func(ctx context.Context, s *testing.State) {
			if err := runSubTest(ctx, func(ctx context.Context, app *cca.App) error {
				return t.testFunc(ctx, app, tconn)
			}, cca.TestWithAppParams{StopAppOnlyIfExist: true}); err != nil {
				s.Errorf("Failed to pass %v subtest: %v", t.name, err)
			}
		})
		cancel()
	}
}

// testPhotoTaking tests if the HDRnet pipeline works as expected by taking a
// photo in Chrome Camera App and check the histogram values.
func testPhotoTaking(ctx context.Context, app *cca.App, tconn *chrome.TestConn) error {
	// There should be some latency for all the processing stages.
	const minProcessingLatency = 1.0
	// There should be no error.
	const hdrnetNoError = 0.0
	// There should be one still shot taken.
	const expectedStillShotsTaken = 1.0
	// For CCA photo mode we should have one or more YUV stream with BLOB
	// depending on the feature set enabled.
	const singleYUVWithBLOB = 1.0
	const multipleYUVWithBLOB = 3.0
	const multipleYUVOfDifferentAspectRatioWithBLOB = 5.0

	histogramTests := histogramutil.HistogramTests{
		"ChromeOS.Camera.HDRnet.AverageLatency.Preprocessing":  histogramutil.AssertHistogramMeanGt(minProcessingLatency),
		"ChromeOS.Camera.HDRnet.AverageLatency.RgbPipeline":    histogramutil.AssertHistogramMeanGt(minProcessingLatency),
		"ChromeOS.Camera.HDRnet.AverageLatency.Postprocessing": histogramutil.AssertHistogramMeanGt(minProcessingLatency),
		"ChromeOS.Camera.HDRnet.Error":                         histogramutil.AssertHistogramEq(hdrnetNoError),
		"ChromeOS.Camera.HDRnet.NumStillShotsTaken":            histogramutil.AssertHistogramEq(expectedStillShotsTaken),
		"ChromeOS.Camera.HDRnet.StreamConfiguration":           histogramutil.AssertHistogramIn(singleYUVWithBLOB, multipleYUVWithBLOB, multipleYUVOfDifferentAspectRatioWithBLOB),
	}

	// Open CCA, take a picture and then close CCA. The histograms are
	// uploaded only when the camera device is closed.
	recorder, err := histogramTests.Record(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to start histogram recorder")
	}
	if err := app.SwitchMode(ctx, cca.Photo); err != nil {
		return errors.Wrap(err, "failed to switch to Photo mode")
	}
	if _, err = app.TakeSinglePhoto(ctx, cca.TimerOff); err != nil {
		return errors.Wrap(err, "failed to take a photo")
	}
	if err = app.Close(ctx); err != nil {
		return errors.Wrap(err, "failed to close Chrome Camera App")
	}
	return histogramTests.Verify(ctx, tconn, recorder)
}

// testVideoRecording tests if the HDRnet pipeline works as expected by
// recording a video in Chrome Camera App and check the histogram values.
func testVideoRecording(ctx context.Context, app *cca.App, tconn *chrome.TestConn) error {
	// There should be some latency for all the processing stages.
	const minProcessingLatency = 1.0
	// There should be no error.
	const hdrnetNoError = 0.0
	// There should be no still shot taken.
	const expectedStillShotsTaken = 0.0
	// For CCA photo mode we should have one or more YUV stream with BLOB
	// depending on the feature set enabled.
	const singleYUVWithBLOB = 1.0
	const multipleYUVWithBLOB = 3.0
	const multipleYUVOfDifferentAspectRatioWithBLOB = 5.0

	histogramTests := histogramutil.HistogramTests{
		"ChromeOS.Camera.HDRnet.AverageLatency.Preprocessing":  histogramutil.AssertHistogramMeanGt(minProcessingLatency),
		"ChromeOS.Camera.HDRnet.AverageLatency.RgbPipeline":    histogramutil.AssertHistogramMeanGt(minProcessingLatency),
		"ChromeOS.Camera.HDRnet.AverageLatency.Postprocessing": histogramutil.AssertHistogramMeanGt(minProcessingLatency),
		"ChromeOS.Camera.HDRnet.Error":                         histogramutil.AssertHistogramEq(hdrnetNoError),
		"ChromeOS.Camera.HDRnet.NumStillShotsTaken":            histogramutil.AssertHistogramEq(expectedStillShotsTaken),
		"ChromeOS.Camera.HDRnet.StreamConfiguration":           histogramutil.AssertHistogramIn(singleYUVWithBLOB, multipleYUVWithBLOB, multipleYUVOfDifferentAspectRatioWithBLOB),
	}

	// Open CCA, record a 5-second video and then close CCA. The histograms
	// are uploaded only when the camera device is closed.
	if err := app.SwitchMode(ctx, cca.Video); err != nil {
		return errors.Wrap(err, "failed to switch to Video mode")
	}
	// Starts histogram recording again after switch to Video mode ignore
	// the histograms generated in previous CCA modes.
	recorder, err := histogramTests.Record(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to start histogram recorder")
	}
	if _, err = app.RecordVideo(ctx, cca.TimerOff, 5*time.Second); err != nil {
		return errors.Wrap(err, "failed to record a video")
	}
	if err = app.Close(ctx); err != nil {
		return errors.Wrap(err, "failed to close Chrome Camera App")
	}
	return histogramTests.Verify(ctx, tconn, recorder)
}
