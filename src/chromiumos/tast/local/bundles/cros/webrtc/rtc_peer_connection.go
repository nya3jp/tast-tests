// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package webrtc

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/local/bundles/cros/webrtc/camera"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/logging"
	"chromiumos/tast/local/media/pre"
	"chromiumos/tast/local/media/videotype"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/webrtc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: RTCPeerConnection,
		Desc: "Verifies that WebRTC RTCPeerConnection in a loopback works",
		Contacts: []string{
			"mcasas@chromium.org", // Test author.
			"chromeos-gfx-video@google.com",
			"chromeos-video-eng@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Data:         append(webrtc.DataFiles(), "loopback_camera.html"),
		Pre:          pre.ChromeVideoWithFakeWebcam(),
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Name: "h264",
			Val:  videotype.H264,
			// "chrome_internal" is needed because H.264 is a proprietary codec.
			ExtraSoftwareDeps: []string{"chrome_internal"},
		}, {
			Name: "vp8",
			Val:  videotype.VP8,
		}, {
			Name: "vp9",
			Val:  videotype.VP9,
		}},
	})
}

// RTCPeerConnection starts a loopback WebRTC call with two RTCPeerConnections
// and ensures it successfully establishes the call (otherwise the test will
// simply fail). If successful, it looks at the video frames coming out on the
// receiving side of the call and looks for freezes and black frames.
func RTCPeerConnection(ctx context.Context, s *testing.State) {
	runPeerConn(ctx, s, s.PreValue().(*chrome.Chrome),
		s.Param().(videotype.Codec), 3*time.Second, camera.VerboseLogging)
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

// setPerf stores performance data of peerConnCameraResult into p.
// codec is a video codec exercised in testing.
func (r *peerConnCameraResult) setPerf(p *perf.Values, codec videotype.Codec) {
	r.FrameStats.SetPerf(p, string(codec))
	r.PeerConnectionStats.setPerf(p, string(codec))
}

// runPeerConn run a test in /data/loopback_camera.html.
// codec is a video codec to exercise in testing.
// duration specifies how long video capturing will run for each resolution.
// If verbose is true, video drivers' verbose messages will be enabled.
// verbose must be false for performance tests.
func runPeerConn(ctx context.Context, s *testing.State, cr *chrome.Chrome,
	codec videotype.Codec, duration time.Duration, verbose camera.VerboseLoggingMode) peerConnCameraResult {
	if verbose == camera.VerboseLogging {
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
