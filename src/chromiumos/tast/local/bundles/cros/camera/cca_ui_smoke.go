// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

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
		Params: []testing.Param{{
			Name:              "real",
			ExtraSoftwareDeps: []string{caps.BuiltinCamera},
			Fixture:           "ccaLaunched",
			ExtraAttr:         []string{"informational"},
			Val:               none,
		}, {
			Name:              "vivid",
			ExtraSoftwareDeps: []string{caps.VividCamera},
			Fixture:           "ccaLaunched",
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel("reven")),
			ExtraAttr:         []string{"group:camera-postsubmit"},
			Val:               none,
		}, {
			Name:    "fake",
			Fixture: "ccaLaunchedWithFakeCamera",
			Val:     none,
		}, {
			Name:    "photo_fake",
			Fixture: "ccaLaunchedWithFakeCamera",
			Val:     photoTaking,
		}, {
			Name:    "video_fake",
			Fixture: "ccaLaunchedWithFakeCamera",
			// TODO(b/191846403): Promote the test to Chrome CQ once it is stable enough.
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"proprietary_codecs"},
			Val:               videoRecoridng,
		}},
	})
}

func CCAUISmoke(ctx context.Context, s *testing.State) {
	app := s.FixtValue().(cca.FixtureData).App()
	testFunction := s.Param().(testFunctionality)
	s.FixtValue().(cca.FixtureData).SetDebugParams(cca.DebugParams{SaveCameraFolderWhenFail: true})

	if testFunction == photoTaking {
		if err := app.SwitchMode(ctx, cca.Photo); err != nil {
			s.Error("Failed to switch to photo mode: ", err)
		}
		if _, err := app.TakeSinglePhoto(ctx, cca.TimerOff); err != nil {
			s.Error("Failed to take photo: ", err)
		}
	} else if testFunction == videoRecoridng {
		if err := app.SwitchMode(ctx, cca.Video); err != nil {
			s.Error("Failed to switch to video mode: ", err)
		}
		if _, err := app.RecordVideo(ctx, cca.TimerOff, 3*time.Second); err != nil {
			s.Error("Failed to record video: ", err)
		}
	}
}
