// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/local/camera/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIMultiCamera,
		Desc:         "Opens CCA and verifies the multi-camera related use cases",
		Contacts:     []string{"inker@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:camera-libcamera"},
		SoftwareDeps: []string{"camera_app", "chrome", caps.BuiltinOrVividCamera},
		Data:         []string{"cca_ui.js"},
		Pre:          chrome.LoggedIn(),
	})
}

func CCAUIMultiCamera(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tb, err := testutil.NewTestBridge(ctx, cr, testutil.UseRealCamera)
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
		if err := tconn.Eval(ctx,
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

		initialFacing, err := app.GetFacing(ctx)
		if err != nil {
			s.Fatal("Get facing failed: ", err)
		}
		if initialFacing == defaultFacing {
			return
		}
		// It may fail to open desired default facing camera with respect to
		// tablet or clamshell mode on device without camera of that facing
		// or on device without facing configurations which returns facing
		// unknown for every camera. Try to query facing from every available
		// camera to ensure it's a true failure.
		if err := app.RunThroughCameras(ctx, func(facing cca.Facing) error {
			facing, err := app.GetFacing(ctx)
			if err != nil {
				return errors.Wrap(err, "failed to get facing")
			}
			if facing == defaultFacing {
				s.Fatalf("Failed to open default camera facing got %v; want %v",
					initialFacing, defaultFacing)
			}
			return nil
		}); err != nil {
			s.Fatal("Failed to get all camera facing: ", err)
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
		if err := app.CheckVisible(ctx, cca.SwitchDeviceButton, false); err != nil {
			s.Fatal("Check switch button failed: ", err)
		}
	} else {
		s.Fatal("No camera found")
	}

	if err := app.Restart(ctx, tb); err != nil {
		var errJS *cca.ErrJS
		if errors.As(err, &errJS) {
			s.Error("There are JS errors when running CCA: ", err)
		} else {
			s.Fatal("Failed to restart CCA: ", err)
		}
	}

	// CCA should still open default camera regardless of what was opened last
	// time.
	checkFacing()
}
