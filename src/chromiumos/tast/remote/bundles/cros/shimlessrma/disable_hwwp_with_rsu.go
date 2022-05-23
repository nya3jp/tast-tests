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

func init() {
	testing.AddTest(&testing.Test{
		Func:         DisableHWWPWithRSU,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Can complete Shimless RMA successfully. Disable HWWP with RSU unlock code",
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
	})
}

func DisableHWWPWithRSU(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	firmwareHelper := s.FixtValue().(*fixture.Value).Helper
	dut := firmwareHelper.DUT
	key := s.RequiredVar("ui.signinProfileTestExtensionManifestKey")

	if err := firmwareHelper.RequireServo(ctx); err != nil {
		s.Fatal("Fail to init servo: ", err)
	}

	uiHelper, err := rmaweb.NewUIHelper(ctx, dut, firmwareHelper, s.RPCHint(), key, false)
	if err != nil {
		s.Fatal("Fail to initialize RMA Helper: ", err)
	}
	defer uiHelper.DisposeResource(cleanupCtx)

	if err := uiHelper.SetupInitStatus(ctx); err != nil {
		s.Fatal("Fail to setup init status: ", err)
	}

	if err := action.Combine("Navigate to RSU page and turn off write protect",
		uiHelper.WelcomePageOperation,
		uiHelper.ComponentsPageOperation,
		uiHelper.OwnerPageOperation,
		uiHelper.WipeDevicePageOperation,
		uiHelper.WriteProtectPageChooseRSU,
		uiHelper.RSUPageOperation,
	)(ctx); err != nil {
		s.Fatal("Fail to navigate to RSU page and turn off write protect: ", err)
	}

	// Wait for reboot start.
	testing.Sleep(ctx, rmaweb.WaitForRebootStart)

	uiHelper, err = rmaweb.NewUIHelper(ctx, dut, firmwareHelper, s.RPCHint(), key, true)
	if err != nil {
		s.Fatal("Fail to initialize RMA Helper: ", err)
	}
	defer uiHelper.DisposeResource(cleanupCtx)

	if err := action.Combine("Navigate to firmware installation page and install firmware",
		uiHelper.WriteProtectDisabledPageOperation,
		uiHelper.BypassFirmwareInstallation,
	)(ctx); err != nil {
		s.Fatal("Fail to navigate to firmware installation page and install firmware: ", err)
	}

	// Wait for reboot start.
	testing.Sleep(ctx, rmaweb.WaitForRebootStart)

	uiHelper, err = rmaweb.NewUIHelper(ctx, dut, firmwareHelper, s.RPCHint(), key, true)
	if err != nil {
		s.Fatal("Fail to initialize RMA Helper: ", err)
	}
	defer uiHelper.DisposeResource(cleanupCtx)

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

	if err := action.Combine("Navigate to Repair Complete page",
		uiHelper.FinalizingRepairPageOperation,
		uiHelper.RepairCompeletedPageOperation,
	)(ctx); err != nil {
		s.Fatal("Fail to navigate to Repair Complete page: ", err)
	}
}
