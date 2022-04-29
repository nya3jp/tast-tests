// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"time"

	"google.golang.org/grpc"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/powercontrol"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ShutdownUsingUI,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies shutdown through UI and boot using power button",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome", "reboot"},
		ServiceDeps:  []string{"tast.cros.browser.ChromeService", "tast.cros.ui.AutomationService"},
		VarDeps:      []string{"servo"},
		Timeout:      8 * time.Minute,
	})
}

func ShutdownUsingUI(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 2*time.Minute)
	defer cancel()

	servoSpec := s.RequiredVar("servo")
	dut := s.DUT()

	pxy, err := servo.NewProxy(ctx, servoSpec, dut.KeyFile(), dut.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(cleanupCtx)

	defer func(ctx context.Context) {
		testing.ContextLog(ctx, "Performing cleanup")
		if !dut.Connected(ctx) {
			if err := powercontrol.PowerOntoDUT(ctx, pxy, dut); err != nil {
				s.Fatal("Failed to power on DUT at cleanup: ", err)
			}
		}
	}(cleanupCtx)

	loginChrome := func() (*rpc.Client, error) {
		cl, err := rpc.Dial(ctx, dut, s.RPCHint())
		if err != nil {
			return nil, errors.Wrap(err, "failed to connect to the RPC service on the DUT")
		}

		// Start Chrome on the DUT.
		cs := ui.NewChromeServiceClient(cl.Conn)
		loginReq := &ui.NewRequest{}
		if _, err := cs.New(ctx, loginReq, grpc.WaitForReady(true)); err != nil {
			s.Fatal("Failed to start Chrome: ", err)
		}
		return cl, nil
	}

	// Perform initial Chrome login.
	cl, err := loginChrome()
	if err != nil {
		s.Fatal("Failed to login Chrome: ", err)
	}

	iter := 5
	for i := 1; i <= iter; i++ {
		uiautoSvc := ui.NewAutomationServiceClient(cl.Conn)
		testing.ContextLogf(ctx, "Iteration: %d/%d", i, iter)
		// Performs some actions on the UI like Opening status tray
		// and perform shutdown with UI button.
		var statusTray = "ash/StatusAreaWidgetDelegate"
		statusTrayFinder := &ui.Finder{
			NodeWiths: []*ui.NodeWith{
				{Value: &ui.NodeWith_HasClass{HasClass: statusTray}},
				// {Value: &ui.NodeWith_First{}},
				{Value: &ui.NodeWith_Role{Role: ui.Role_ROLE_PANE}},
			},
		}
		if _, err := uiautoSvc.WaitUntilExists(ctx, &ui.WaitUntilExistsRequest{Finder: statusTrayFinder}); err != nil {
			s.Fatal("Failed to find status tray: ", err)
		}
		if _, err := uiautoSvc.LeftClick(ctx, &ui.LeftClickRequest{Finder: statusTrayFinder}); err != nil {
			s.Fatal("Failed to click status tray: ", err)
		}

		shutdownButtonFinder := &ui.Finder{
			NodeWiths: []*ui.NodeWith{
				{Value: &ui.NodeWith_Name{Name: "Shut down"}},
				{Value: &ui.NodeWith_Role{Role: ui.Role_ROLE_BUTTON}},
			},
		}
		if _, err := uiautoSvc.WaitUntilExists(ctx, &ui.WaitUntilExistsRequest{Finder: shutdownButtonFinder}); err != nil {
			s.Fatal("Failed to find shutdown button on DUT UI: ", err)
		}
		if _, err := uiautoSvc.LeftClick(ctx, &ui.LeftClickRequest{Finder: shutdownButtonFinder}); err != nil {
			s.Fatal("Failed to click UI shutdown button: ", err)
		}

		sdCtx, cancel := context.WithTimeout(ctx, 40*time.Second)
		defer cancel()
		if err := dut.WaitUnreachable(sdCtx); err != nil {
			s.Fatal("Failed to wait for the DUT being unreachable: ", err)
		}

		if err := powercontrol.PowerOntoDUT(ctx, pxy, dut); err != nil {
			s.Fatal("Failed to power on DUT: ", err)
		}

		// Perfoming prev_sleep_state check.
		expectedPrevSleepState := 5
		if err := powercontrol.ValidatePrevSleepState(ctx, dut, expectedPrevSleepState); err != nil {
			s.Fatal("Failed to validate previous sleep state: ", err)
		}

		// Perform Chrome login after powering on from shutdown.
		cl, err = loginChrome()
		if err != nil {
			s.Fatal("Failed to login Chrome after shutdown: ", err)
		}
	}
}
