// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/video/lib/caps"
	"chromiumos/tast/local/chrome"
//	"chromiumos/tast/local/bundles/cros/video/webrtc"
	"chromiumos/tast/testing"
)

const FAKE_CAMERA_STREAM_FILE = "crowd720_25frames.y4m"
const CCA_ID = "hfhhnacclhffhdffklopdkcgdhifgngh"

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromeCameraApp,
		Desc:         "Verifies that CCA can open",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{caps.USBCamera, "chrome_login", "camera_720p"},
		Data:         append([]string{FAKE_CAMERA_STREAM_FILE}),
	})
}

// WebRTCCamera makes WebRTC getUserMedia call and renders the camera's media
// stream in a video tag. It will test VGA and 720p and check if the gUM call succeeds.
// This test will fail when an error occurs or too many frames are broken.
//
// WebRTCCamera performs video capturing for 3 seconds. It is a short version of
// video.WebRTCCameraPerf.
//
// This test uses the real webcam unless it is running under QEMU. Under QEMU,
// it uses "vivid" instead, which is the virtual video test driver and can be
// used as an external USB camera.
func ChromeCameraApp(ctx context.Context, s *testing.State) {
	fake_camera_stream_file_path := s.DataPath(FAKE_CAMERA_STREAM_FILE)
  chromeArgs := []string{
    "--use-fake-ui-for-media-stream",
    "--use-fake-device-for-media-stream",
    "--use-file-for-fake-video-capture=" + fake_camera_stream_file_path,
    "--enable-experimental-web-platform-features",
  }
	cr, err := chrome.New(ctx, chrome.ExtraArgs(chromeArgs...))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)


	conn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatalf("Failed to connect to test API: %v", err)
	}
	defer conn.Close()

	launched_app := false
	if err := conn.EvalPromise(ctx,
		`new Promise((resolve, reject) => {
			chrome.autotestPrivate.launchApp("hfhhnacclhffhdffklopdkcgdhifgngh", () => {
	      			resolve(true);
    			});
		})`, &launched_app); err != nil {
		s.Fatal("Failed to call chrome.autotestPrivate.launchApp: ", err)
	}
	s.Log("launched_app = ", launched_app)


	
	bgURL := chrome.ExtensionBackgroundPageURL(CCA_ID)
	s.Log("Connecting to CCA ", bgURL)
	cca_conn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(bgURL))
	

	//fgURL := "chrome-extension://" + CCA_ID + "/views/main.html"
	//cca_conn, err := cr.NewConn(ctx, fgURL)
	if err != nil {
		s.Fatal("Failed to connect to CCA: ", err)
	}

  time.Sleep(5000000000)
	s.Log("Connected to CCA: ", cca_conn)
	
	html := ""
	if err := cca_conn.EvalPromise(ctx,
		`
      //new Promise((resolve, reject) => {
			//resolve(cca.bg.MIN_WIDTH);
      //chrome.chromeosInfoPrivate.get(['board'],
      //      (values) => resolve(values['board']));
      cca.bg.send("aaa")
		  //})
    `, &html); err != nil {
		s.Fatal("Failed to call CCA foreground page methods: ", err)
	}
	s.Log("body class list = ", html)
}
