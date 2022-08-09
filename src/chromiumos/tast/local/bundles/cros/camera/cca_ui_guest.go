// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/local/mountns"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIGuest,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks camera app can be launched in guest mode",
		Contacts:     []string{"pihsun@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:camera-libcamera"},
		SoftwareDeps: []string{"camera_app", "chrome", caps.BuiltinOrVividCamera},
		Fixture:      "ccaLaunchedGuest",
	})
}

func CCAUIGuest(ctx context.Context, s *testing.State) {
	app := s.FixtValue().(cca.FixtureData).App()
	s.FixtValue().(cca.FixtureData).SetDebugParams(cca.DebugParams{SaveCameraFolderWhenFail: true})

	// Enter user session mount namespace so the captured video and photo can
	// be checked by the test.
	// TODO(b/229131841): Move this to the fixture when tast supports forcing
	// the SetUp / TearDown function running in the same thread as the test,
	// since entering user session mount namespace is only effective on the
	// same thread.
	if err := mountns.WithUserSessionMountNS(ctx, func(ctx context.Context) error {
		if err := app.SwitchMode(ctx, cca.Photo); err != nil {
			s.Error("Failed to switch to photo mode: ", err)
		}
		if _, err := app.TakeSinglePhoto(ctx, cca.TimerOff); err != nil {
			s.Error("Failed to take photo: ", err)
		}
		if err := app.SwitchMode(ctx, cca.Video); err != nil {
			s.Error("Failed to switch to video mode: ", err)
		}
		if _, err := app.RecordVideo(ctx, cca.TimerOff, 3*time.Second); err != nil {
			s.Error("Failed to record video: ", err)
		}
		return nil
	}); err != nil {
		s.Fatal("Failed entering user session mount namespace: ", err)
	}
}
