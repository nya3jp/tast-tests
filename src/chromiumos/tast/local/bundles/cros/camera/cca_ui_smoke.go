// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/local/camera/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

type smokeTestParams struct {
	useCameraType testutil.UseCameraType
	function      testFunctionality
}

type testFunctionality int

const (
	none testFunctionality = iota
	photoTaking
	videoRecoridng
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUISmoke,
		Desc:         "Smoke test for Chrome Camera App",
		Contacts:     []string{"inker@chromium.org", "shik@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "group:camera-libcamera"},
		SoftwareDeps: []string{"camera_app", "chrome"},
		Data:         []string{"cca_ui.js"},
		Params: []testing.Param{{
			Name:              "real",
			ExtraSoftwareDeps: []string{caps.BuiltinCamera},
			Pre:               chrome.LoggedIn(),
			ExtraAttr:         []string{"informational"},
			Val: smokeTestParams{
				useCameraType: testutil.UseRealCamera,
				function:      none,
			},
		}, {
			Name:              "vivid",
			ExtraSoftwareDeps: []string{caps.VividCamera},
			Pre:               chrome.LoggedIn(),
			ExtraAttr:         []string{"group:camera-postsubmit"},
			Val: smokeTestParams{
				useCameraType: testutil.UseVividCamera,
				function:      none,
			},
		}, {
			Name: "fake",
			Pre:  testutil.ChromeWithFakeCamera(),
			Val: smokeTestParams{
				useCameraType: testutil.UseFakeCamera,
				function:      none,
			},
		}, {
			Name: "photo_fake",
			Pre:  testutil.ChromeWithFakeCamera(),
			// TODO(b/191846403): Promote the test to Chrome CQ once it is stable enough.
			ExtraAttr: []string{"informational"},
			Val: smokeTestParams{
				useCameraType: testutil.UseFakeCamera,
				function:      photoTaking,
			},
		}, {
			Name: "video_fake",
			Pre:  testutil.ChromeWithFakeCamera(),
			// TODO(b/191846403): Promote the test to Chrome CQ once it is stable enough.
			ExtraAttr: []string{"informational"},
			Val: smokeTestParams{
				useCameraType: testutil.UseFakeCamera,
				function:      videoRecoridng,
			},
		}},
	})
}

func CCAUISmoke(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	testParams := s.Param().(smokeTestParams)
	tb, err := testutil.NewTestBridge(ctx, cr, testParams.useCameraType)
	if err != nil {
		s.Fatal("Failed to construct test bridge: ", err)
	}
	defer tb.TearDown(ctx)

	if err := cca.ClearSavedDirs(ctx, cr); err != nil {
		s.Fatal("Failed to clear saved directory: ", err)
	}

	app, err := cca.New(ctx, cr, []string{s.DataPath("cca_ui.js")}, s.OutDir(), tb)
	if err != nil {
		s.Fatal("Failed to open CCA: ", err)
	}
	defer func(ctx context.Context) {
		if err := app.Close(ctx); err != nil {
			s.Error("Failed to close app: ", err)
		}
	}(ctx)

	if testParams.function == photoTaking {
		if err := app.SwitchMode(ctx, cca.Photo); err != nil {
			s.Error("Failed to switch to photo mode: ", err)
		}
		if _, err := app.TakeSinglePhoto(ctx, cca.TimerOff); err != nil {
			s.Error("Failed to take photo: ", err)
		}
	} else if testParams.function == videoRecoridng {
		if err := app.SwitchMode(ctx, cca.Video); err != nil {
			s.Error("Failed to switch to video mode: ", err)
		}
		if _, err := app.RecordVideo(ctx, cca.TimerOff, 3*time.Second); err != nil {
			s.Error("Failed to record video: ", err)
		}
	}
}
