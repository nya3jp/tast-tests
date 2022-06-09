// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package shimlessrma contains integration tests for Shimless RMA SWA.
package shimlessrma

import (
	"context"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/bundles/cros/shimlessrma/rmaweb"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type param struct {
	wp          rmaweb.WriteProtectDisableOption
	enroll      bool
	destination rmaweb.DestinationOption
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         DisableHWWP,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Can complete Shimless RMA successfully. Disable HWWP with Battery Disconnection",
		Contacts: []string{
			"yanghenry@google.com",
			"chromeos-engprod-syd@google.com",
		},
		Attr: []string{"group:shimless_rma", "shimless_rma_experimental"},
		VarDeps: []string{
			"ui.signinProfileTestExtensionManifestKey",
		},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
		ServiceDeps:  []string{"tast.cros.browser.ChromeService", "tast.cros.shimlessrma.AppService"},
		Fixture:      fixture.NormalMode,
		Timeout:      10 * time.Minute,
		Params: []testing.Param{{
			Name: "unenroll_sameuser_manual",
			Val: param{
				wp:          rmaweb.Manual,
				enroll:      false,
				destination: rmaweb.SameUser,
			},
		}, {
			Name: "unenroll_sameuser_rsu",
			Val: param{
				wp:          rmaweb.Rsu,
				enroll:      false,
				destination: rmaweb.DifferentUser,
			},
		}, {
			Name: "enroll_diffuser_rsu",
			Val: param{
				wp:          rmaweb.Rsu,
				enroll:      true,
				destination: rmaweb.DifferentUser,
			},
		}},
	})
}

func DisableHWWP(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	firmwareHelper := s.FixtValue().(*fixture.Value).Helper
	dut := firmwareHelper.DUT
	key := s.RequiredVar("ui.signinProfileTestExtensionManifestKey")
	p := s.Param().(param)
	wpOption := p.wp
	enroll := p.enroll
	destination := p.destination

	if err := firmwareHelper.RequireServo(ctx); err != nil {
		s.Fatal("Fail to init servo: ", err)
	}

	uiHelper, err := rmaweb.NewUIHelper(ctx, dut, firmwareHelper, s.RPCHint(), key, false)
	if err != nil {
		s.Fatal("Fail to initialize RMA Helper: ", err)
	}
	// Restart will dispose resources, so don't dispose resources explicitly.

	if err := uiHelper.SetupInitStatus(ctx, enroll); err != nil {
		s.Fatal("Fail to setup init status: ", err)
	}

	if err := generateActionCombinedToDisableWP(wpOption, enroll, destination, uiHelper)(ctx); err != nil {
		s.Fatal("Fail to navigate to Disable Write Protect page and turn off write protect: ", err)
	}

	// Wait for reboot start.
	testing.Sleep(ctx, rmaweb.WaitForRebootStart)

	uiHelper, err = rmaweb.NewUIHelper(ctx, dut, firmwareHelper, s.RPCHint(), key, true)
	if err != nil {
		s.Fatal("Fail to initialize RMA Helper: ", err)
	}
	// Restart will dispose resources, so don't dispose resources explicitly.

	if err := action.Combine("Navigate to firmware installation page and install firmware",
		uiHelper.WriteProtectDisabledPageOperation,
		uiHelper.WaitForFirmwareInstallation,
	)(ctx); err != nil {
		s.Fatal("Fail to navigate to firmware installation page and install firmware: ", err)
	}

	// Wait for reboot start.
	testing.Sleep(ctx, rmaweb.WaitForRebootStart)

	uiHelper, err = rmaweb.NewUIHelper(ctx, dut, firmwareHelper, s.RPCHint(), key, true)
	if err != nil {
		s.Fatal("Fail to initialize RMA Helper: ", err)
	}
	// Restart will dispose resources, so don't dispose resources explicitly.

	if err := action.Combine("Navigate to Device Provision page",
		uiHelper.FirmwareInstallationPageOperation,
		uiHelper.DeviceInformationPageOperation,
		uiHelper.DeviceProvisionPageOperation,
	)(ctx); err != nil {
		s.Fatal("Fail to navigate to Device Provision page: ", err)
	}

	// Another reboot after provisioning
	testing.Sleep(ctx, rmaweb.WaitForRebootStart)

	uiHelper, err = rmaweb.NewUIHelper(ctx, dut, firmwareHelper, s.RPCHint(), key, true)
	if err != nil {
		s.Fatal("Fail to initialize RMA Helper: ", err)
	}
	defer uiHelper.DisposeResource(cleanupCtx)

	// TODO: I comment out the following code due to a bug in Shimless RMA.
	// I will add it back after Shimless RMA fix it.
	// Bug link: b:231906070
	/**
	web.WriteProtectEnabledPageOperation(ctx, s, client)
	s.Log("WriteProtectEnabledPageOperation")

	// Wait for reboot start.
	testing.Sleep(ctx, rmaweb.WaitForRebootStart)

	cl, client, error = rmaweb.CreateShimlessClient(ctx, s.RPCHint(), dut, firmwareHelper, s.RequiredVar("ui.signinProfileTestExtensionManifestKey"), true)
	defer cl.Close(cleanupCtx)
	defer client.CloseShimlessRMA(cleanupCtx, &empty.Empty{})
	if err != nil {
		s.Fatal("Fail to create Shimless RMA Client: ", err)
	}
	s.Log("Init Shimless RMA successfully after enable CCD")
	*/

	if err := action.Combine("Navigate to Repair Complete page",
		uiHelper.FinalizingRepairPageOperation,
		uiHelper.RepairCompletedPageOperation,
	)(ctx); err != nil {
		s.Fatal("Fail to navigate to Repair Complete page: ", err)
	}
}

func generateActionCombinedToDisableWP(option rmaweb.WriteProtectDisableOption, enroll bool, destination rmaweb.DestinationOption, uiHelper *rmaweb.UIHelper) action.Action {
	if enroll {
		return action.Combine("Navigate to RSU page and turn off write protect",
			uiHelper.WelcomePageOperation,
			uiHelper.ComponentsPageOperation,
			uiHelper.OwnerPageOperation(destination),
			uiHelper.RSUPageOperation,
		)
	}

	if option == rmaweb.Manual {
		return action.Combine("Navigate to Manual Disable Write Protect page and turn off write protect",
			uiHelper.WelcomePageOperation,
			uiHelper.ComponentsPageOperation,
			uiHelper.OwnerPageOperation(destination),
			uiHelper.WipeDevicePageOperation,
			uiHelper.WriteProtectPageChooseManual,
		)
	}

	return action.Combine("Navigate to RSU page and turn off write protect",
		uiHelper.WelcomePageOperation,
		uiHelper.ComponentsPageOperation,
		uiHelper.OwnerPageOperation(destination),
		// uiHelper.WipeDevicePageOperation,
		uiHelper.WriteProtectPageChooseRSU,
		uiHelper.RSUPageOperation,
	)
}
