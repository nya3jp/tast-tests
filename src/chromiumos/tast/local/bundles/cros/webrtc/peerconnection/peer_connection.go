// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package peerconnection provides common code for webrtc.* RTCPeerConnection tests.
package peerconnection

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/webrtc/camera"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/logging"
	"chromiumos/tast/local/media/videotype"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/webrtc"
	"chromiumos/tast/testing"
)

// CodecType is the type of codec to check for.
type CodecType int

const (
	// Encoding refers to WebRTC video encoding.
	Encoding CodecType = iota
	// Decoding refers to WebRTC video decoding.
	Decoding
)

// RunRTCPeerConnectionAccelUsed launches a loopback RTCPeerConnection and inspects that the
// CodecType codec is hardware accelerated.
func RunRTCPeerConnectionAccelUsed(ctx context.Context, s *testing.State, codecType CodecType, profile string) {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		s.Fatal("Failed to set values for verbose logging")
	}
	defer vl.Close()

	chromeArgs := webrtc.ChromeArgsWithFakeCameraInput(true)
	if codecType == Encoding {
		if profile == "H264" {
			// Vaapi H264 Encoder is disabled on grunt by default on Chrome. Enable the feature by the command line option.
			// TODO(b/145961243): Remove this option when VA-API H264 encoder is
			// enabled on grunt by default.
			chromeArgs = append(chromeArgs, "--enable-features=VaapiH264AMDEncoder")
		}
	}

	cr, err := chrome.New(ctx, chrome.ExtraArgs(chromeArgs...))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	conn, err := cr.NewConn(ctx, server.URL+"/"+webrtc.LoopbackPage)
	if err != nil {
		s.Fatal("Failed to open video page: ", err)
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	if err := conn.WaitForExpr(ctx, "document.readyState === 'complete'"); err != nil {
		s.Fatal("Timed out waiting for page loading: ", err)
	}

	if err := conn.EvalPromise(ctx, fmt.Sprintf("start(%q)", profile), nil); err != nil {
		s.Fatal("Error establishing connection: ", err)
	}

	if err := checkForCodecImplementation(ctx, s, conn, codecType); err != nil {
		s.Fatal("checkForCodecImplementation() failed: ", err)
	}
}

// checkForCodecImplementation parses the RTCPeerConnection and verifies that it
// is using hardware acceleration for codecType. This method uses the
// RTCPeerConnection getStats() API [1].
// [1] https://w3c.github.io/webrtc-pc/#statistics-model
func checkForCodecImplementation(ctx context.Context, s *testing.State, conn *chrome.Conn, codecType CodecType) error {
	// See [1] and [2] for the statNames to use here. The values are browser
	// specific, for Chrome, "External{Deco,Enco}der" means that WebRTC is using
	// hardware acceleration and anything else (e.g. "libvpx", "ffmpeg",
	// "unknown") means it is not.
	// [1] https://w3c.github.io/webrtc-stats/#dom-rtcinboundrtpstreamstats-decoderimplementation
	// [2] https://w3c.github.io/webrtc-stats/#dom-rtcoutboundrtpstreamstats-encoderimplementation
	statName := "encoderImplementation"
	peerConnectionName := "localPeerConnection"
	expectedImplementation := "ExternalEncoder"
	if codecType == Decoding {
		statName = "decoderImplementation"
		peerConnectionName = "remotePeerConnection"
		expectedImplementation = "ExternalDecoder"
	}

	parseStatsJS :=
		fmt.Sprintf(`new Promise(function(resolve, reject) {
			%s.getStats(null).then(stats => {
				if (stats == null) {
					reject("getStats() failed");
					return;
				}
				stats.forEach(report => {
					Object.keys(report).forEach(statName => {
						if (statName === '%s') {
							resolve(report[statName]);
							return;
						}
					})
				})
				reject("%s not found");
			});
		})`, peerConnectionName, statName, statName)

	// Poll getStats() to wait until expectedImplementation gets filled in:
	// RTCPeerConnection needs a few frames to start up encoding/decoding; in the
	// meantime it returns "unknown".
	const pollInterval = 100 * time.Millisecond
	const pollTimeout = 200 * pollInterval
	var implementation string
	err := testing.Poll(ctx,
		func(ctx context.Context) error {
			if err := conn.EvalPromise(ctx, parseStatsJS, &implementation); err != nil {
				return errors.Wrap(err, "failed to retrieve and/or parse RTCStatsReport")
			}
			if implementation == "unknown" {
				return errors.New("getStats() didn't fill in the codec implementation (yet)")
			}
			return nil
		}, &testing.PollOptions{Interval: pollInterval, Timeout: pollTimeout})

	if err != nil {
		return err
	}
	s.Logf("%s: %s", statName, implementation)

	if implementation != expectedImplementation {
		return errors.Errorf("unexpected implementation, got %v, expected %v", implementation, expectedImplementation)
	}
	return nil
}

// peerConnectionStats is a struct used in peerConnCameraResult for FPS data.
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

// peerConnCameraResult is a struct for decoding JSON objects obtained from /data/loopback_camera.html.
type peerConnCameraResult struct {
	CameraType          string              `json:"cameraType"`
	PeerConnectionStats peerConnectionStats `json:"peerConnectionStats"`
	FrameStats          camera.FrameStats   `json:"frameStats"`
	Errors              []string            `json:"errors"`
}

// SetPerf stores performance data of peerConnCameraResult into p.
// codec is a video codec exercised in testing.
func (r *peerConnCameraResult) SetPerf(p *perf.Values, codec videotype.Codec) {
	r.FrameStats.SetPerf(p, string(codec))
	r.PeerConnectionStats.setPerf(p, string(codec))
}

// VerboseLoggingMode describes whether video driver's verbose debug log is enabled.
type VerboseLoggingMode int

const (
	// VerboseLogging enables verbose logging.
	VerboseLogging VerboseLoggingMode = iota
	// NoVerboseLogging disables verbose logging.
	NoVerboseLogging
)

// RunRTCPeerConnection run a test in /data/loopback_camera.html.
// codec is a video codec to exercise in testing.
// duration specifies how long video capturing will run for each resolution.
// If verbose is true, video drivers' verbose messages will be enabled.
// verbose must be false for performance tests.
func RunRTCPeerConnection(ctx context.Context, s *testing.State, cr *chrome.Chrome,
	codec videotype.Codec, duration time.Duration, verbose VerboseLoggingMode) peerConnCameraResult {
	if verbose == VerboseLogging {
		vl, err := logging.NewVideoLogger()
		if err != nil {
			s.Fatal("Failed to set values for verbose logging")
		}
		defer vl.Close()
	}

	var result peerConnCameraResult
	camera.RunTest(ctx, s, cr, "loopback_camera.html",
		fmt.Sprintf("testWebRtcLoopbackCall('%s', %d)", codec, duration/time.Second), &result)

	s.Logf("Result: %+v", result)

	if len(result.Errors) != 0 {
		for _, msg := range result.Errors {
			s.Error("Error: ", msg)
		}
	}
	if err := result.FrameStats.CheckTotalFrames(); err != nil {
		s.Error("Video was not healthy: ", err)
	}
	if err := result.FrameStats.CheckBrokenFrames(); err != nil {
		s.Error("Video was not healthy: ", err)
	}

	return result
}
