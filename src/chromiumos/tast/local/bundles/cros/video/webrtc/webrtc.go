// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package webrtc provides common code for video.WebRTC* tests.
package webrtc

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/video/lib/logging"
	"chromiumos/tast/local/bundles/cros/video/lib/videotype"
	"chromiumos/tast/local/bundles/cros/video/lib/vm"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/testing"
)

// DataFiles returns a list of required files that tests that use this package
// should include in their Data fields.
func DataFiles() []string {
	return []string{
		"third_party/blackframe.js",
		"third_party/munge_sdp.js",
		"third_party/ssim.js",
	}
}

// runTest checks if the given WebRTC tests work correctly.
// htmlName is a filename of an HTML file in data directory.
// entryPoint is a JavaScript expression that starts the test there.
func runTest(ctx context.Context, s *testing.State, cr *chrome.Chrome,
	htmlName, entryPoint string, results interface{}) {

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	conn, err := cr.NewConn(ctx, server.URL+"/"+htmlName)
	if err != nil {
		s.Fatal("Creating renderer failed: ", err)
	}
	defer conn.Close()

	if err := conn.WaitForExpr(ctx, "scriptReady"); err != nil {
		s.Fatal("Timed out waiting for scripts ready: ", err)
	}

	if err := conn.WaitForExpr(ctx, "checkVideoInput()"); err != nil {
		var msg string
		if err := conn.Eval(ctx, "enumerateDevicesError", &msg); err != nil {
			s.Error("Failed to evaluate enumerateDevicesError: ", err)
		} else if len(msg) > 0 {
			s.Error("enumerateDevices failed: ", msg)
		}
		s.Fatal("Timed out waiting for video device to be available: ", err)
	}

	if err := conn.Exec(ctx, entryPoint); err != nil {
		s.Fatal("Failed to start test: ", err)
	}

	rctx, rcancel := ctxutil.Shorten(ctx, 3*time.Second)
	defer rcancel()
	if err := conn.WaitForExpr(rctx, "isTestDone"); err != nil {
		// If test didn't finish within the deadline, display error messages stored in "globalErrors".
		var errors []string
		if err := conn.Eval(ctx, "globalErrors", &errors); err == nil {
			for _, msg := range errors {
				s.Error("Got JS error: ", msg)
			}
		}
		s.Fatal("Timed out waiting for test completed: ", err)
	}

	if err := conn.Eval(ctx, "getResults()", results); err != nil {
		s.Fatal("Failed to get results from JS: ", err)
	}
}

func percentage(num, total int) float64 {
	if total == 0 {
		return 100.0
	}
	return 100.0 * float64(num) / float64(total)
}

// frameStats is a struct for statistics of frames.
type frameStats struct {
	TotalFrames  int `json:"totalFrames"`
	BlackFrames  int `json:"blackFrames"`
	FrozenFrames int `json:"frozenFrames"`
}

// blackFramesPercentage returns the ratio of black frames to total frames
func (s *frameStats) blackFramesPercentage() float64 {
	return percentage(s.BlackFrames, s.TotalFrames)
}

// frozenFramesPercentage returns the ratio of frozen frames to total frames
func (s *frameStats) frozenFramesPercentage() float64 {
	return percentage(s.FrozenFrames, s.TotalFrames)
}

// checkVideoHealth checks if video frames were healthy.
// We basically check whether a video frame was displayed.
// If the test ran under QEMU, we also check the ratio of broken frames.
// This is because we are free from hardware flakiness in that case.
func (s *frameStats) checkVideoHealth() error {
	if s.TotalFrames == 0 {
		return errors.New("no frame was displayed")
	}

	// If the test was running under QEMU, check the percentage of broken frames.
	if vm.IsRunningOnVM() {
		// Ratio of broken frames must be less than |threshold| %.
		const threshold = 1.0
		blackPercentage := s.blackFramesPercentage()
		frozenPercentage := s.frozenFramesPercentage()
		if threshold < blackPercentage+frozenPercentage {
			return errors.Errorf("too many broken frames: black %.1f%%, frozen %.1f%% (total %d)",
				blackPercentage, frozenPercentage, s.TotalFrames)
		}
	}

	return nil
}

// setPerf records performance data in frameStats to perf.Values.
// p is a pointer for perf.Values where data will be stored.
// suffix is a string that will be used as sufixes of metrics' names.
func (s *frameStats) setPerf(p *perf.Values, suffix string) {
	blackFrames := perf.Metric{
		Name:      "tast_black_frames_percentage_" + suffix,
		Unit:      "percent",
		Direction: perf.SmallerIsBetter,
	}
	frozenFrames := perf.Metric{
		Name:      "tast_frozen_frames_percentage_" + suffix,
		Unit:      "percent",
		Direction: perf.SmallerIsBetter,
	}

	p.Set(blackFrames, s.blackFramesPercentage())
	p.Set(frozenFrames, s.frozenFramesPercentage())
}

// CameraResults is a type for decoding JSON objects obtained from /video/data/getusermedia.html.
type CameraResults []struct {
	Width      int        `json:"width"`
	Height     int        `json:"height"`
	FrameStats frameStats `json:"frameStats"`
	Errors     []string   `json:"errors"`
}

// SetPerf stores performance data of CameraResults into p.
func (r *CameraResults) SetPerf(p *perf.Values) {
	for _, result := range *r {
		perfSuffix := fmt.Sprintf("%dx%d", result.Width, result.Height)
		result.FrameStats.setPerf(p, perfSuffix)
	}
}

// RunWebRTCCamera run a test in /video/data/getusermedia.html.
// duration is an integer that specify how many seconds video capturing will run in for each resolution.
func RunWebRTCCamera(ctx context.Context, s *testing.State, cr *chrome.Chrome,
	duration time.Duration) CameraResults {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		s.Fatal("Failed to set values for verbose logging")
	}
	defer vl.Close()

	var results CameraResults
	runTest(ctx, s, cr, "getusermedia.html", fmt.Sprintf("testNextResolution(%d)", duration/time.Second), &results)

	s.Logf("Results: %+v", results)

	for _, result := range results {
		if len(result.Errors) != 0 {
			for _, msg := range result.Errors {
				s.Errorf("%dx%d: %s", result.Width, result.Height, msg)
			}
		}

		if err := result.FrameStats.checkVideoHealth(); err != nil {
			s.Errorf("%dx%d was not healthy: %v", result.Width, result.Height, err)
		}
	}

	return results
}

// peerConnectionStats is a struct used in PeerConnCameraResult for FPS data.
type peerConnectionStats struct {
	MinInFPS      float64 `json:"minInFps"`
	MaxInFPS      float64 `json:"maxInFps"`
	AverageInFPS  float64 `json:"averageInFps"`
	MinOutFPS     float64 `json:"minOutFps"`
	MaxOutFPS     float64 `json:"maxOutFps"`
	AverageOutFPS float64 `json:"averageOutFps"`
}

// setPerf stores performance data of peerConnectionStats into p.
// suffix is a string that will be used as a sufix in metric names.
func (s *peerConnectionStats) setPerf(p *perf.Values, suffix string) {
	maxInFPS := perf.Metric{
		Name:      "tast_max_input_fps_" + suffix,
		Unit:      "fps",
		Direction: perf.BiggerIsBetter,
	}
	maxOutFPS := perf.Metric{
		Name:      "tast_max_output_fps_" + suffix,
		Unit:      "fps",
		Direction: perf.BiggerIsBetter,
	}

	p.Set(maxInFPS, s.MaxInFPS)
	p.Set(maxOutFPS, s.MaxOutFPS)
}

// PeerConnCameraResult is a struct for decoding JSON objects obtained from /video/data/loopback_camera.html.
type PeerConnCameraResult struct {
	CameraType          string              `json:"cameraType"`
	PeerConnectionStats peerConnectionStats `json:"peerConnectionStats"`
	FrameStats          frameStats          `json:"frameStats"`
	Errors              []string            `json:"errors"`
}

// SetPerf stores performance data of PeerConnCameraResult into p.
// codec is a video codec exercised in testing.
func (r *PeerConnCameraResult) SetPerf(p *perf.Values, codec videotype.Codec) {
	r.FrameStats.setPerf(p, string(codec))
	r.PeerConnectionStats.setPerf(p, string(codec))
}

// RunWebRTCPeerConnCamera run a test in /video/data/loopback_camera.html.
// duration is an integer that specify how many seconds video capturing will run in for each resolution.
// codec is a video codec to exercise in testing.
func RunWebRTCPeerConnCamera(ctx context.Context, s *testing.State, cr *chrome.Chrome,
	codec videotype.Codec, duration time.Duration) PeerConnCameraResult {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		s.Fatal("Failed to set values for verbose logging")
	}
	defer vl.Close()

	var result PeerConnCameraResult
	runTest(ctx, s, cr, "loopback_camera.html",
		fmt.Sprintf("testWebRtcLoopbackCall('%s', %d)", codec, duration/time.Second), &result)

	s.Logf("Result: %+v", result)

	if len(result.Errors) != 0 {
		for _, msg := range result.Errors {
			s.Error("Error: ", msg)
		}
	}

	if err := result.FrameStats.checkVideoHealth(); err != nil {
		s.Error("Video was not healthy: ", err)
	}

	return result
}
