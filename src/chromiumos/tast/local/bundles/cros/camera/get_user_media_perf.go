// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/common/perf"
	"chromiumos/tast/local/bundles/cros/camera/getusermedia"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/lacros"
	"chromiumos/tast/local/lacros/launcher"
	"chromiumos/tast/local/media/pre"
	"chromiumos/tast/local/webrtc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         GetUserMediaPerf,
		Desc:         "Captures performance data about getUserMedia video capture",
		Contacts:     []string{"shik@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{caps.BuiltinOrVividCamera, "chrome", "camera_720p"},
		Data:         append(webrtc.DataFiles(), "getusermedia.html"),
		Params: []testing.Param{
			{
				Pre: pre.ChromeCameraPerf(),
				Val: lacros.ChromeTypeChromeOS,
			},
			{
				Name:              "lacros",
				Fixture:           "chromeCameraPerfLacros",
				ExtraSoftwareDeps: []string{"lacros"},
				Timeout:           7 * time.Minute, // A lenient limit for launching Lacros Chrome.
				Val:               lacros.ChromeTypeLacros,
			},
		},
	})
}

// GetUserMediaPerf is the full version of GetUserMedia. It renders the camera's
// media stream in VGA and 720p for 20 seconds. If there is no error while
// exercising the camera, it uploads statistics of black/frozen frames. This
// test will fail when an error occurs or too many frames are broken.
//
// This test uses the real webcam unless it is running under QEMU. Under QEMU,
// it uses "vivid" instead, which is the virtual video test driver and can be
// used as an external USB camera.
func GetUserMediaPerf(ctx context.Context, s *testing.State) {
	var cr getusermedia.ChromeInterface
	runLacros := s.Param().(lacros.ChromeType) == lacros.ChromeTypeLacros
	if runLacros {
		var err error
		cr, err = launcher.LaunchLacrosChrome(ctx, s.FixtValue().(launcher.FixtData))
		if err != nil {
			s.Fatal("Failed to launch lacros-chrome: ", err)
		}
		defer cr.Close(ctx)
	} else {
		cr = s.PreValue().(*chrome.Chrome)
	}

	// Run tests for 20 seconds per resolution.
	results := getusermedia.RunGetUserMedia(ctx, s, cr, 20*time.Second, getusermedia.NoVerboseLogging)

	if !s.HasError() {
		// Set and upload frame statistics below.
		p := perf.NewValues()
		results.SetPerf(p)
		if err := p.Save(s.OutDir()); err != nil {
			s.Error("Failed saving perf data: ", err)
		}
	}
}
