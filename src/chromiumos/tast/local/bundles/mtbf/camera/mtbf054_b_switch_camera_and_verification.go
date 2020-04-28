// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"time"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/bundles/mtbf/camera/cca"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBF054BSwitchCameraAndVerification,
		Desc:         "Verification for MTBF054ModesFallback",
		Contacts:     []string{"xliu@cienet.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", caps.BuiltinOrVividCamera},
		Data:         []string{"cca_ui.js"},
		Pre:          chrome.LoginReuse(),
	})
}

// MTBF054BSwitchCameraAndVerification switch camera and
// verify the portrait mode icon should disappear and
// mode selector will auto fallback to photo mode.
func MTBF054BSwitchCameraAndVerification(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	app, err := cca.GetConnection(ctx, cr, []string{s.DataPath("cca_ui.js")})
	if err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.CmrOpenCCA, err))
	}
	defer app.Close(ctx)

	if err := app.WaitForVideoActive(ctx); err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.CmrInact, err))
	}
	s.Log("Preview started")

	// Plugin usb external camera may delay connection
	testing.Sleep(ctx, 3*time.Second)

	numCameras, err := app.GetNumOfCameras(ctx)
	if err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.CmrNumber, err))
	}

	if numCameras > 1 {
		// Switch camera.
		if err := app.SwitchCamera(ctx); err != nil {
			s.Fatal(mtbferrors.New(mtbferrors.CmrSwitch, err))
		}
	} else if numCameras == 1 {
		s.Fatal(mtbferrors.New(mtbferrors.CmrCameraNum, nil))
	} else {
		s.Fatal(mtbferrors.New(mtbferrors.CmrNotFound, err))
	}
	if err := app.WaitForVideoActive(ctx); err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.CmrInact, err))
	}

	// Check mode selector fallback to photo mode.
	if active, err := app.GetState(ctx, string(cca.Photo)); err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.CmrAppState, err))
	} else if !active {
		s.Fatal(mtbferrors.New(mtbferrors.CmrFallBack, nil))
	}

	// Check the portrait mode icon should disappear
	const portraitModeSelector = "Tast.isVisible('#modes-group > .mode-item:last-child')"
	if err := app.CheckElementExist(ctx, portraitModeSelector, false); err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.CmrPortraitBtn, err))
	}
}
