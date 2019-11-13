// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/local/bundles/cros/webrtc/camera"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/logging"
	"chromiumos/tast/local/media/pre"
	"chromiumos/tast/local/media/vm"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/webrtc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         GetUserMedia,
		Desc:         "Verifies that getUserMedia captures video",
		Contacts:     []string{"shik@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{caps.BuiltinOrVividCamera, "chrome", "camera_720p"},
		Pre:          pre.ChromeVideo(),
		Data:         append(webrtc.DataFiles(), "getusermedia.html"),
	})
}

// GetUserMedia calls getUserMedia call and renders the camera's media stream
// in a video tag. It will test VGA and 720p and check if the gUM call succeeds.
// This test will fail when an error occurs or too many frames are broken.
//
// GetUserMedia performs video capturing for 3 seconds with 480p and 720p.
// (It's 10 seconds in case it runs under QEMU.) This a short version of
// camera.GetUserMediaPerf.
//
// This test uses the real webcam unless it is running under QEMU. Under QEMU,
// it uses "vivid" instead, which is the virtual video test driver and can be
// used as an external USB camera. In this case, the time limit is 10 seconds.
func GetUserMedia(ctx context.Context, s *testing.State) {
	duration := 3 * time.Second
	// Since we use vivid on VM and it's slower than real cameras,
	// we use a longer time limit: https://crbug.com/929537
	if vm.IsRunningOnVM() {
		duration = 10 * time.Second
	}

	// Run tests for 480p and 720p.
	runGetUserMedia(ctx, s, s.PreValue().(*chrome.Chrome), duration,
		camera.VerboseLogging)
}

// cameraResults is a type for decoding JSON objects obtained from /data/getusermedia.html.
type cameraResults []struct {
	Width      int               `json:"width"`
	Height     int               `json:"height"`
	FrameStats camera.FrameStats `json:"frameStats"`
	Errors     []string          `json:"errors"`
}

// setPerf stores performance data of cameraResults into p.
func (r *cameraResults) setPerf(p *perf.Values) {
	for _, result := range *r {
		perfSuffix := fmt.Sprintf("%dx%d", result.Width, result.Height)
		result.FrameStats.SetPerf(p, perfSuffix)
	}
}

// runGetUserMedia run a test in /data/getusermedia.html.
// duration specifies how long video capturing will run for each resolution.
// If verbose is true, video drivers' verbose messages will be enabled.
// verbose must be false for performance tests.
func runGetUserMedia(ctx context.Context, s *testing.State, cr *chrome.Chrome,
	duration time.Duration, verbose camera.VerboseLoggingMode) cameraResults {
	if verbose == camera.VerboseLogging {
		vl, err := logging.NewVideoLogger()
		if err != nil {
			s.Fatal("Failed to set values for verbose logging")
		}
		defer vl.Close()
	}

	var results cameraResults
	camera.RunTest(ctx, s, cr, "getusermedia.html", fmt.Sprintf("testNextResolution(%d)", duration/time.Second), &results)

	s.Logf("Results: %+v", results)

	for _, result := range results {
		if len(result.Errors) != 0 {
			for _, msg := range result.Errors {
				s.Errorf("%dx%d: %s", result.Width, result.Height, msg)
			}
		}

		if err := result.FrameStats.CheckTotalFrames(); err != nil {
			s.Errorf("%dx%d was not healthy: %v", result.Width, result.Height, err)
		}
		// Only check the percentage of broken and black frames if we are
		// running under QEMU, see crbug.com/898745.
		if vm.IsRunningOnVM() {
			if err := result.FrameStats.CheckBrokenFrames(); err != nil {
				s.Errorf("%dx%d was not healthy: %v", result.Width, result.Height, err)
			}
		}
	}

	return results
}
