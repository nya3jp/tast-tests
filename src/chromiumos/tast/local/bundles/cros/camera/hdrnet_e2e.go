// Copyright 2021 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         HDRnetE2E,
		LacrosStatus: testing.LacrosVariantUnknown,
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

type histogramVerifier func(m *metrics.Histogram) error
type histogramTests map[string]histogramVerifier

// names returns the names of the histograms tracked by |ht| as string slice.
func (ht histogramTests) names() []string {
	names := make([]string, len(ht))
	i := 0
	for v := range ht {
		names[i] = v
		i++
	}
	return names
}

// record starts recording changes of the histograms tracked by |ht|.
func (ht histogramTests) record(ctx context.Context, tconn *chrome.TestConn) (*metrics.Recorder, error) {
	metrics.ClearHistogramTransferFile()
	return metrics.StartRecorder(ctx, tconn, ht.names()...)
}

// wait waits until the histograms tracked by |ht| have new values.
func (ht histogramTests) wait(ctx context.Context, tconn *chrome.TestConn, recorder *metrics.Recorder) ([]*metrics.Histogram, error) {
	// It takes time for Chrome to refresh the histograms. Leave 3 seconds
	// for the clean-up tasks in case of error.
	ctxDeadline, _ := ctx.Deadline()
	histTimeout := ctxDeadline.Sub(time.Now()) - 3*time.Second
	return recorder.WaitAll(ctx, tconn, histTimeout)
}

// verify waits until the histograms tracked by |ht| are ready and verifies the
// histogram values recorded using the HistogramVerifier associated with each
// histogram.
func (ht histogramTests) verify(ctx context.Context, tconn *chrome.TestConn, recorder *metrics.Recorder) error {
	histograms, err := ht.wait(ctx, tconn, recorder)
	if err != nil {
		return errors.Wrap(err, "failed to get updated histograms")
	}

	for _, h := range histograms {
		if err := ht[h.Name](h); err != nil {
			// Dump the whole histogram diff so that we get a better
			// idea of what else also failed.
			testing.ContextLog(ctx, "Histogram diff: ", histograms)
			return err
		}
	}

	return nil
}

// assertHistogramEq returns a HistogramVerifier that can be used to check if a
// histogram's value equals to |value|.
func assertHistogramEq(value float64) histogramVerifier {
	return func(m *metrics.Histogram) error {
		// We assume that there's only one sample hence we can check
		// against the histogram mean.
		if len(m.Buckets) != 1 {
			return errors.Errorf("invalid %s: %v", m.Name, m.Buckets)
		}
		if mean, err := m.Mean(); err != nil {
			return errors.Wrap(err, "failed to get histogram mean")
		} else if mean != value {
			return errors.Errorf("unexpected value of %s: %v does not equal to %v", m.Name, mean, value)
		}
		return nil
	}
}

// assertHistogramMeanGt returns a HistogramVerifier that can be used to check
// if a histogram's mean value is greater than |value|.
func assertHistogramMeanGt(value float64) histogramVerifier {
	return func(m *metrics.Histogram) error {
		if len(m.Buckets) == 0 {
			return errors.Errorf("invalid %s: %v", m.Name, m.Buckets)
		}
		if mean, err := m.Mean(); err != nil {
			return errors.Wrap(err, "failed to get histogram mean")
		} else if mean <= value {
			return errors.Errorf("unexpected mean of %s: %v is not greater than %v", m.Name, mean, value)
		}
		return nil
	}
}

// testPhotoTaking tests if the HDRnet pipeline works as expected by taking a
// photo in Chrome Camera App and check the histogram values.
func testPhotoTaking(ctx context.Context, app *cca.App, tconn *chrome.TestConn) error {
	// There should be some latency for all the processing stages.
	const minProcessingLatency = 1.0
	// There should be no error.
	const hdrnetNoError = 0.0
	// There should be two concurrent streams.
	const expectedConcurrentStreams = 2.0
	// There should be one still shot taken.
	const expectedStillShotsTaken = 1.0
	// For CCA photo mode we should have one YUV stream with BLOB.
	const singleYUVWithBLOB = 1.0

	histogramTests := histogramTests{
		"ChromeOS.Camera.HDRnet.AverageLatency.Preprocessing":  assertHistogramMeanGt(minProcessingLatency),
		"ChromeOS.Camera.HDRnet.AverageLatency.RgbPipeline":    assertHistogramMeanGt(minProcessingLatency),
		"ChromeOS.Camera.HDRnet.AverageLatency.Postprocessing": assertHistogramMeanGt(minProcessingLatency),
		"ChromeOS.Camera.HDRnet.Error":                         assertHistogramEq(hdrnetNoError),
		"ChromeOS.Camera.HDRnet.NumConcurrentStreams":          assertHistogramEq(expectedConcurrentStreams),
		"ChromeOS.Camera.HDRnet.NumStillShotsTaken":            assertHistogramEq(expectedStillShotsTaken),
		"ChromeOS.Camera.HDRnet.StreamConfiguration":           assertHistogramEq(singleYUVWithBLOB),
	}

	// Open CCA, take a picture and then close CCA. The histograms are
	// uploaded only when the camera device is closed.
	recorder, err := histogramTests.record(ctx, tconn)
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
	return histogramTests.verify(ctx, tconn, recorder)
}

// testVideoRecording tests if the HDRnet pipeline works as expected by
// recording a video in Chrome Camera App and check the histogram values.
func testVideoRecording(ctx context.Context, app *cca.App, tconn *chrome.TestConn) error {
	// There should be some latency for all the processing stages.
	const minProcessingLatency = 1.0
	// There should be no error.
	const hdrnetNoError = 0.0
	// There should be two concurrent streams.
	const expectedConcurrentStreams = 2.0
	// There should be no still shot taken.
	const expectedStillShotsTaken = 0.0
	// For CCA video mode we should have one YUV stream with BLOB.
	const singleYUVWithBLOB = 1.0

	histogramTests := histogramTests{
		"ChromeOS.Camera.HDRnet.AverageLatency.Preprocessing":  assertHistogramMeanGt(minProcessingLatency),
		"ChromeOS.Camera.HDRnet.AverageLatency.RgbPipeline":    assertHistogramMeanGt(minProcessingLatency),
		"ChromeOS.Camera.HDRnet.AverageLatency.Postprocessing": assertHistogramMeanGt(minProcessingLatency),
		"ChromeOS.Camera.HDRnet.Error":                         assertHistogramEq(hdrnetNoError),
		"ChromeOS.Camera.HDRnet.NumConcurrentStreams":          assertHistogramEq(expectedConcurrentStreams),
		"ChromeOS.Camera.HDRnet.NumStillShotsTaken":            assertHistogramEq(expectedStillShotsTaken),
		"ChromeOS.Camera.HDRnet.StreamConfiguration":           assertHistogramEq(singleYUVWithBLOB),
	}

	// Open CCA, record a 5-second video and then close CCA. The histograms
	// are uploaded only when the camera device is closed.
	if err := app.SwitchMode(ctx, cca.Video); err != nil {
		return errors.Wrap(err, "failed to switch to Video mode")
	}
	// Starts histogram recording again after switch to Video mode ignore
	// the histograms generated in previous CCA modes.
	recorder, err := histogramTests.record(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to start histogram recorder")
	}
	if _, err = app.RecordVideo(ctx, cca.TimerOff, 5*time.Second); err != nil {
		return errors.Wrap(err, "failed to record a video")
	}
	if err = app.Close(ctx); err != nil {
		return errors.Wrap(err, "failed to close Chrome Camera App")
	}
	return histogramTests.verify(ctx, tconn, recorder)
}
