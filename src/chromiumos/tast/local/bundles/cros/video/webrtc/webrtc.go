// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package webrtc provides common code for video.WebRTC* tests.
package webrtc

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// DataFiles returns a list of required files that tests that use this package
// should include in their Data fields.
func DataFiles() []string {
	return []string{
		"third_party/blackframe.js",
		"third_party/ssim.js",
	}
}

// isVM returns true if the test is running under QEMU.
func isVM(s *testing.State) bool {
	const path = "/sys/devices/virtual/dmi/id/sys_vendor"
	content, err := ioutil.ReadFile(path)

	if err != nil {
		return false
	}

	vendor := strings.TrimSpace(string(content))
	s.Logf("%s : %s", path, vendor)

	return vendor == "QEMU"
}

// loadVivid loads the "vivid" kernel module, which emulates a video capture device.
func loadVivid(ctx context.Context) error {
	cmd := testexec.CommandContext(ctx, "modprobe", "vivid", "n_devs=1", "node_types=0x1")

	if err := cmd.Run(); err != nil {
		cmd.DumpLog(ctx)
		return errors.Wrap(err, "modprobe failed")
	}

	return nil
}

// unloadVivid unloads the "vivid" kernel module.
func unloadVivid(ctx context.Context) error {
	// Use Poll instead of executing modprobe once, because modprobe may fail
	// if it is called before the device is completely released from camera HAL.
	return testing.Poll(ctx, func(ctx context.Context) error {
		cmd := testexec.CommandContext(ctx, "modprobe", "-r", "vivid")

		if err := cmd.Run(); err != nil {
			cmd.DumpLog(ctx)
			return errors.Wrap(err, "modprobe -r failed")
		}
		return nil
	}, nil)
}

// runTest checks if the given WebRTC tests work correctly.
// htmlName is a filename of an HTML file in data directory.
// entryPoint is a JavaScript expression that starts the test there.
func runTest(ctx context.Context, s *testing.State, htmlName, entryPoint string, results interface{}) {

	if isVM(s) {
		s.Log("Loading vivid")
		if err := loadVivid(ctx); err != nil {
			s.Fatal("Failed to load vivid: ", err)
		}
		defer func() {
			s.Log("Unloading vivid")
			if err := unloadVivid(ctx); err != nil {
				s.Fatal("Failed to unload vivid: ", err)
			}
		}()
	}

	cr, err := chrome.New(ctx, chrome.ExtraArgs([]string{"--use-fake-ui-for-media-stream"}))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

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
			s.Error("Failed to evaluate gotEnumerateDevicesError: ", err)
		} else if len(msg) > 0 {
			s.Error("enumerateDevices failed: ", msg)
		}
		s.Fatal("Timed out waiting for video device to be available: ", err)
	}

	if err := conn.Exec(ctx, entryPoint); err != nil {
		s.Fatal("Failed to start test: ", err)
	}

	if err := conn.WaitForExpr(ctx, "isTestDone"); err != nil {
		s.Fatal("Timed out waiting for test completed: ", err)
	}

	if err := conn.Eval(ctx, "getResults()", results); err != nil {
		s.Fatal("Cannot get the value 'results': ", err)
	}
}

func ratio(num, total int) float64 {
	if total == 0 {
		return 1.0
	}
	return float64(num) / float64(total)
}

// frameStats is a struct for statistics of frames.
type frameStats struct {
	TotalFrames  int `json:"numFrames"`
	BlackFrames  int `json:"numBlackFrames"`
	FrozenFrames int `json:"numFrozenFrames"`
}

// blackFramesRatio returns the ratio of black frames to total frames
func (stats *frameStats) blackFramesRatio() float64 {
	return ratio(stats.BlackFrames, stats.TotalFrames)
}

// frozenFramesRatio returns the ratio of frozen frames to total frames
func (stats *frameStats) frozenFramesRatio() float64 {
	return ratio(stats.FrozenFrames, stats.TotalFrames)
}

// CheckVideoHealth checks ratios of broken frames during video capturing.
func (stats *frameStats) CheckVideoHealth() error {
	// Ratio of broken frames must be less than |threshold|
	const threshold = 0.01

	if stats.TotalFrames == 0 {
		return errors.New("no frame was displayed")
	}

	if threshold < stats.blackFramesRatio()+stats.frozenFramesRatio() {
		return errors.Errorf("too many broken frames: black %.1f%%, frozen %.1f%% (total %d)",
			stats.blackFramesRatio(), stats.frozenFramesRatio(), stats.TotalFrames)
	}

	return nil
}

// setPerf records performance data in frameStats to perf.Values.
// p is a pointer for perf.Values where data will be stored.
// suffix is a string that will be used as sufixes of metics' names.
func (stats *frameStats) setPerf(p *perf.Values, suffix string) {
	blackFrames := perf.Metric{
		Name:      fmt.Sprintf("tast_black_frames_percentage_%s", suffix),
		Unit:      "percent",
		Direction: perf.SmallerIsBetter,
	}
	frozenFrames := perf.Metric{
		Name:      fmt.Sprintf("tast_frozen_frames_percentage_%s", suffix),
		Unit:      "percent",
		Direction: perf.SmallerIsBetter,
	}

	p.Set(blackFrames, stats.blackFramesRatio())
	p.Set(frozenFrames, stats.frozenFramesRatio())
}

// webRTCCameraResults is a type for decoding JSONs obtained from /video/data/getusermedia.html.
type webRTCCameraResults []struct {
	Width      int        `json:"width"`
	Height     int        `json:"height"`
	FrameStats frameStats `json:"frameStats"`
	Errors     []string   `json:"cameraErrors"`
}

// SetPerf records performance data in webRTCPeerCameraResults to perf.Values.
// p is a pointer for perf.Values where data will be stored.
func (results *webRTCCameraResults) SetPerf(p *perf.Values) {
	for _, result := range *results {
		perf_suffix := fmt.Sprintf("%dx%d", result.Width, result.Height)
		result.FrameStats.setPerf(p, perf_suffix)
	}
}

// RunWebRTCCamera run a test in /video/data/getusermedia.html.
// duration is an integer that specify how many seconds video capturing will run in for each resolution.
func RunWebRTCCamera(ctx context.Context, s *testing.State, duration int) webRTCCameraResults {
	var results webRTCCameraResults
	runTest(ctx, s, "getusermedia.html", fmt.Sprintf("testNextResolution(%d)", duration), &results)

	s.Logf("Results: %#v", results)

	for _, result := range results {
		if len(result.Errors) != 0 {
			for _, msg := range result.Errors {
				s.Errorf("Error for %dx%d: %s",
					result.Width, result.Height, msg)
			}
		}

		if err := result.FrameStats.CheckVideoHealth(); err != nil {
			s.Errorf("Video was not healthy for %dx%d: %v",
				result.Width, result.Height, err)
		}
	}

	return results
}

// peerConnectionStats is a struct used in webRTCPeerConnectionWithCameraResult for FPS data.
type peerConnectionStats struct {
	MinInFPS      float64 `json: "minInFps"`
	MaxInFPS      float64 `json: "maxInFps"`
	AverageInFPS  float64 `json: "averageInFps"`
	MinOutFPS     float64 `json: "minOutFps"`
	MaxOutFPS     float64 `json: "maxOutFps"`
	AverageOutFPS float64 `json: "averageOutFps"`
}

// setPerf records performance data in peerConnectionStats to perf.Values.
// p is a pointer for perf.Values where data will be stored.
// suffix is a string that will be used as sufixes of metics' names.
func (stats *peerConnectionStats) setPerf(p *perf.Values, suffix string) {
	maxInFPS := perf.Metric{
		Name:      fmt.Sprintf("tast_max_input_fps_%s", suffix),
		Unit:      "fps",
		Direction: perf.BiggerIsBetter,
	}
	maxOutFPS := perf.Metric{
		Name:      fmt.Sprintf("tast_max_output_fps_%s", suffix),
		Unit:      "fps",
		Direction: perf.BiggerIsBetter,
	}

	p.Set(maxInFPS, stats.MaxInFPS)
	p.Set(maxOutFPS, stats.MaxOutFPS)
}

// webRTCPeerConnectionWithCameraResult is a struct for decoding JSONs obtained from /video/data/loopback.html.
type webRTCPeerConnectionWithCameraResult struct {
	CameraType          string              `json:"cameraType"`
	PeerConnectionStats peerConnectionStats `json:"peerConnectionStats"`
	FrameStats          frameStats          `json:"frameStats"`
	Errors              []string            `json:"errors"`
}

// SetPerf records performance data in webRTCPeerConnectionWithCameraResult to perf.Values.
// p is a pointer for perf.Values where data will be stored.
// codec is a string representing video codec. e.g. "VP8", "H264".
func (result *webRTCPeerConnectionWithCameraResult) SetPerf(p *perf.Values, codec string) {
	result.FrameStats.setPerf(p, codec)
	result.PeerConnectionStats.setPerf(p, codec)
}

// RunWebRTCPeerConnectionWithCamera run a test in /video/d2ata/loopback.html.
// duration is an integer that specify how many seconds video capturing will run in for each resolution.
// codec is a string representing video codec. e.g. "VP8", "H264".
func RunWebRTCPeerConnectionWithCamera(
	ctx context.Context, s *testing.State, codec string, duration int) webRTCPeerConnectionWithCameraResult {
	var result webRTCPeerConnectionWithCameraResult
	runTest(ctx, s, "loopback.html", fmt.Sprintf("testWebRtcLoopbackCall('%s', %d)", codec, duration), &result)

	s.Logf("Result: %#v", result)

	if len(result.Errors) != 0 {
		for _, msg := range result.Errors {
			s.Errorf("Error: %s", msg)
		}
	}

	if err := result.FrameStats.CheckVideoHealth(); err != nil {
		s.Errorf("Video was not healthy: %v", err)
	}

	return result
}
