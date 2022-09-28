// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/testing"
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
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Smoke test for Chrome Camera App",
		Contacts:     []string{"pihsun@chromium.org", "shik@chromium.org", "chromeos-camera-eng@google.com"},
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
			// TODO(b/209833758): Removed from CQ due to flake in VM.
			ExtraAttr: []string{"group:camera-postsubmit", "informational"},
			Val:       none,
		}, {
			Name:    "fake",
			Fixture: "ccaLaunchedWithFakeCamera",
			Val:     none,
		}, {
			Name:    "photo_fake",
			Fixture: "ccaLaunchedWithFakeCamera",
			Val:     photoTaking,
			// TODO(b/209833758): Removed from CQ due to flake in VM.
			ExtraAttr: []string{"informational"},
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
