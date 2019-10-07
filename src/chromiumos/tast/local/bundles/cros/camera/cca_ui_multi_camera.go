// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"

	"chromiumos/tast/local/bundles/cros/camera/cca"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIMultiCamera,
		Desc:         "Opens CCA and verifies the multi-camera related use cases",
		Contacts:     []string{"shik@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", caps.BuiltinOrVividCamera},
		Data:         []string{"cca_ui.js"},
		Pre:          chrome.LoggedIn(),
	})
}

func CCAUIMultiCamera(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	app, err := cca.New(ctx, cr, []string{s.DataPath("cca_ui.js")})
	if err != nil {
		s.Fatal("Failed to open CCA: ", err)
	}
	defer app.Close(ctx)

	if err := app.WaitForVideoActive(ctx); err != nil {
		s.Fatal("Preview is inactive after launching App: ", err)
	}
	s.Log("Preview started")

	numCameras, err := app.GetNumOfCameras(ctx)
	if err != nil {
		s.Fatal("Can't get number of cameras: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)

	checkFacing := func() {
		// If it is a VM, there is no need to check the camera facing since it don't
		// have any builtin cameras.
		if vm.IsRunningOnVM() {
			return
		}

		// CCA should open back camera as default if the device is under tablet
		// mode and open front camera as default for clamshell mode.
		var isTabletMode bool
		if err := tconn.EvalPromise(ctx,
			`tast.promisify(chrome.autotestPrivate.isTabletModeEnabled)()`,
			&isTabletMode); err != nil {
			s.Fatal("Failed to recognize device mode: ", err)
		}
		var defaultFacing cca.Facing
		if isTabletMode {
			defaultFacing = cca.FacingBack
		} else {
			defaultFacing = cca.FacingFront
		}

		if err := app.CheckFacing(ctx, defaultFacing); err != nil {
			s.Fatal("Check facing failed: ", err)
		}
	}

	checkFacing()

	if numCameras > 1 {
		// Set grid option.
		gridEnabled, err := app.ToggleGridOption(ctx)
		if err != nil {
			s.Fatal("Toggle grid option failed: ", err)
		}

		// Switch camera.
		if err := app.SwitchCamera(ctx); err != nil {
			s.Fatal("Switch camera failed: ", err)
		}

		// Verify that grid option state is persistent.
		if err := app.CheckGridOption(ctx, gridEnabled); err != nil {
			s.Fatal("Check grid option failed: ", err)
		}
	} else if numCameras == 1 {
		if err := app.CheckSwitchDeviceButtonExist(ctx, false); err != nil {
			s.Fatal("Check switch button failed: ", err)
		}
	} else {
		s.Fatal("No camera found")
	}

	if err := app.Restart(ctx); err != nil {
		s.Fatal("Failed to restart CCA: ", err)
	}

	if err := app.WaitForVideoActive(ctx); err != nil {
		s.Fatal("Preview is inactive after launching App: ", err)
	}

	// CCA should still open default camera regardless of what was opened last
	// time.
	checkFacing()
}
