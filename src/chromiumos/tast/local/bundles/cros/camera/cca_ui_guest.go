// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIGuest,
		LacrosStatus: testing.LacrosVariantUnknown,
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

	if err := app.SwitchMode(ctx, cca.Photo); err != nil {
		s.Error("Failed to switch to photo mode: ", err)
	}
	if _, err := app.TakeSinglePhoto(ctx, cca.TimerOff); err != nil {
		s.Error("Failed to take photo: ", err)
	}
}
