// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/bundles/mtbf/camera/cca"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBF043SwitchCamera,
		Desc:         "Opens CCA and verifies the multi-camera switching",
		Contacts:     []string{"xliu@cienet.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", caps.BuiltinOrVividCamera},
		Data:         []string{"cca_ui.js"},
		Pre:          chrome.LoginReuse(),
	})
}

func MTBF043SwitchCamera(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	app, err := cca.New(ctx, cr, []string{s.DataPath("cca_ui.js")}, s.OutDir())
	if err != nil {
		if strings.Contains(err.Error(), "Chrome probably crashed") {
			s.Fatal(mtbferrors.New(mtbferrors.CmrChromeCrashed, err))
		}
		s.Fatal(mtbferrors.New(mtbferrors.CmrOpenCCA, err))
	}
	defer app.Close(ctx)

	if err := app.WaitForVideoActive(ctx); err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.CmrInact, err))
	}
	s.Log("Preview started")

	numCameras, err := app.GetNumOfCameras(ctx)
	if err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.CmrNumber, err))
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
			s.Fatal(mtbferrors.New(mtbferrors.CmrDevMode, err))
		}
		var defaultFacing cca.Facing
		if isTabletMode {
			defaultFacing = cca.FacingBack
		} else {
			defaultFacing = cca.FacingFront
		}

		if err := app.CheckFacing(ctx, defaultFacing); err != nil {
			s.Fatal(mtbferrors.New(mtbferrors.CmrFacing, err))
		}
	}

	checkFacing()

	if numCameras > 1 {
		// Switch camera.
		if err := app.SwitchCamera(ctx); err != nil {
			s.Fatal(mtbferrors.New(mtbferrors.CmrSwitch, err))
		}
	} else if numCameras == 1 {
		if err := app.CheckSwitchDeviceButtonExist(ctx, false); err != nil {
			s.Fatal(mtbferrors.New(mtbferrors.CmrSwitchBtn, err))
		}
	} else {
		s.Fatal(mtbferrors.New(mtbferrors.CmrNotFound, err))
	}
	testing.Sleep(ctx, 3*time.Second)
}
