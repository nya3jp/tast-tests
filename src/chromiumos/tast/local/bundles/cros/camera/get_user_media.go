// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/local/bundles/cros/camera/getusermedia"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/lacros/launcher"
	"chromiumos/tast/local/media/pre"
	"chromiumos/tast/local/media/vm"
	"chromiumos/tast/local/webrtc"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         GetUserMedia,
		Desc:         "Verifies that getUserMedia captures video",
		Contacts:     []string{"shik@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "group:camera-libcamera"},
		SoftwareDeps: []string{"chrome"},
		Data:         append(webrtc.DataFiles(), "getusermedia.html"),
		Params: []testing.Param{
			{
				Name:              "real",
				Pre:               pre.ChromeVideo(),
				ExtraAttr:         []string{"informational"},
				ExtraSoftwareDeps: []string{caps.BuiltinCamera, "camera_720p"},
				Val:               lacros.ChromeTypeChromeOS,
			},
			{
				Name:              "vivid",
				Pre:               pre.ChromeVideo(),
				ExtraAttr:         []string{"informational"},
				ExtraSoftwareDeps: []string{caps.VividCamera},
				Val:               lacros.ChromeTypeChromeOS,
			},
			{
				Name:              "fake",
				Pre:               pre.ChromeVideoWithFakeWebcam(),
				ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel("reven")),
				Val:               lacros.ChromeTypeChromeOS,
			},
			{
				Name:              "lacros",
				Fixture:           "chromeVideoLacros",
				ExtraAttr:         []string{"informational"},
				ExtraSoftwareDeps: []string{caps.BuiltinOrVividCamera, "camera_720p", "lacros"},
				Timeout:           7 * time.Minute, // A lenient limit for launching Lacros Chrome.
				Val:               lacros.ChromeTypeLacros,
			},
		},
	})
}

// GetUserMedia calls getUserMedia call and renders the camera's media stream
// in a video tag. It will test VGA and 720p and check if the gUM call succeeds.
// This test will fail when an error occurs or too many frames are broken.
//
// GetUserMedia performs video capturing for 3 seconds with 480p and 720p.
// (It's 10 seconds in case it runs under QEMU.) This a short version of
// camera.GetUserMediaPerf.
func GetUserMedia(ctx context.Context, s *testing.State) {
	duration := 3 * time.Second
	// Since we use vivid on VM and it's slower than real cameras,
	// we use a longer time limit: https://crbug.com/929537
	if vm.IsRunningOnVM() {
		duration = 10 * time.Second
	}

	var cr getusermedia.ChromeInterface
	if s.Param().(lacros.ChromeType) == lacros.ChromeTypeLacros {
		var err error
		cr, err = launcher.LaunchLacrosChrome(ctx, s.FixtValue().(launcher.FixtData))
		if err != nil {
			s.Fatal("Failed to launch lacros-chrome: ", err)
		}
		defer cr.Close(ctx)
	} else {
		cr = s.PreValue().(*chrome.Chrome)
	}
	// Run tests for 480p and 720p.
	getusermedia.RunGetUserMedia(ctx, s, cr, duration, getusermedia.VerboseLogging)
}
