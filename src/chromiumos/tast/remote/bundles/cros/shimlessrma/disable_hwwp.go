// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package shimlessrma contains integration tests for Shimless RMA SWA.
package shimlessrma

import (
	"context"
	"strconv"
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
		Attr: []string{"group:shimless_rma"},
		VarDeps: []string{
			"ui.signinProfileTestExtensionManifestKey",
		},
		Vars:         []string{"firmware.skipFlashUSB"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
		ServiceDeps:  []string{"tast.cros.browser.ChromeService", "tast.cros.shimlessrma.AppService"},
		Fixture:      fixture.NormalMode,
		Timeout:      30 * time.Minute,
		Params: []testing.Param{{
			ExtraAttr: []string{"shimless_rma_normal"},
			Name:      "unenroll_sameuser_manual",
			Val: param{
				wp:          rmaweb.Manual,
				enroll:      false,
				destination: rmaweb.SameUser,
			},
		}, {
			ExtraAttr: []string{"shimless_rma_nodelocked"},
			Name:      "unenroll_sameuser_rsu",
			Val: param{
				wp:          rmaweb.Rsu,
				enroll:      false,
				destination: rmaweb.SameUser,
			},
		}, {
			ExtraAttr: []string{"shimless_rma_nodelocked"},
			Name:      "unenroll_diffuser_rsu",
			Val: param{
				wp:          rmaweb.Rsu,
				enroll:      false,
				destination: rmaweb.DifferentUser,
			},
		}, {
			ExtraAttr: []string{"shimless_rma_nodelocked"},
			Name:      "enroll_diffuser_rsu",
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

	s.Log("Setup USB Key")
	skipFlashUSB := false
	if skipFlashUSBStr, ok := s.Var("firmware.skipFlashUSB"); ok {
		var err error
		skipFlashUSB, err = strconv.ParseBool(skipFlashUSBStr)
		if err != nil {
			s.Fatalf("Invalid value for var firmware.skipFlashUSB: got %q, want true/false", skipFlashUSBStr)
		}
	}
	s.Logf("skipFlashUSB is %t", skipFlashUSB)

	// USB install only occurs in Manual option.
	// So always skip it for RSU testing.
	if !skipFlashUSB && wpOption == rmaweb.Manual {
		s.Log("Flash USB starts")
		cs := s.CloudStorage()
		if err := firmwareHelper.SetupUSBKey(ctx, cs); err != nil {
			s.Fatal("USBKey not working: ", err)
		}
	}

	uiHelper, err := rmaweb.NewUIHelper(ctx, dut, firmwareHelper, s.RPCHint(), key, false)
	if err != nil {
		s.Fatal("Fail to initialize RMA Helper: ", err)
	}
	// Restart will dispose resources, so don't dispose resources explicitly.

	if err := uiHelper.SetupInitStatus(ctx, enroll); err != nil {
		s.Fatal("Fail to setup init status: ", err)
	}

	if actions := generateActionCombinedToDisableWP(wpOption, enroll, destination, uiHelper); actions == nil {
		// We don't support this test case yet.
		s.Fatalf("The test case is not support yet. Enroll: %t, WP: %s, destination: %s ", enroll, wpOption, destination)
	} else {
		if err := actions(ctx); err != nil {
			s.Fatal("Fail to navigate to Disable Write Protect page and turn off write protect: ", err)
		}
	}

	// Wait for reboot start.
	if err := testing.Sleep(ctx, rmaweb.WaitForRebootStart); err != nil {
		s.Error("Fail to sleep: ", err)
	}

	uiHelper, err = rmaweb.NewUIHelper(ctx, dut, firmwareHelper, s.RPCHint(), key, true)
	if err != nil {
		s.Fatal("Fail to initialize RMA Helper: ", err)
	}
	// Restart will dispose resources, so don't dispose resources explicitly.

	// faft-cr50-pool cannot update firmware from USB.
	// Since we already run Manual test case (removal battery) in skylab and install firmware from USB,
	// we skip firmware installation in all RSU test cases.
	if wpOption == rmaweb.Manual {
		if err := action.Combine("navigate to firmware installation page and install firmware",
			uiHelper.WriteProtectDisabledPageOperation,
			uiHelper.WaitForFirmwareInstallation,
		)(ctx); err != nil {
			s.Fatal("Fail to navigate to firmware installation page and install firmware: ", err)
		}
	} else {
		if err := action.Combine("navigate to firmware installation page and bypass firmware install",
			uiHelper.WriteProtectDisabledPageOperation,
			uiHelper.BypassFirmwareInstallation,
		)(ctx); err != nil {
			s.Fatal("Fail to navigate to firmware installation page and bypass firmware install: ", err)
		}
	}

	// Wait for reboot start.
	if err := testing.Sleep(ctx, rmaweb.WaitForRebootStart); err != nil {
		s.Error("Fail to sleep: ", err)
	}

	uiHelper, err = rmaweb.NewUIHelper(ctx, dut, firmwareHelper, s.RPCHint(), key, true)
	if err != nil {
		s.Fatal("Fail to initialize RMA Helper: ", err)
	}
	// Restart will dispose resources, so don't dispose resources explicitly.

	if err := action.Combine("navigate to Device Provision page",
		uiHelper.FirmwareInstallationPageOperation,
		uiHelper.DeviceInformationPageOperation,
		uiHelper.DeviceProvisionPageOperation,
	)(ctx); err != nil {
		s.Fatal("Fail to navigate to Device Provision page: ", err)
	}

	// Another reboot after provisioning
	if err := testing.Sleep(ctx, rmaweb.WaitForRebootStart); err != nil {
		s.Error("Fail to sleep: ", err)
	}

	uiHelper, err = rmaweb.NewUIHelper(ctx, dut, firmwareHelper, s.RPCHint(), key, true)
	if err != nil {
		s.Fatal("Fail to initialize RMA Helper: ", err)
	}
	defer uiHelper.DisposeResource(cleanupCtx)

	storeLogFlag := rmaweb.NotStoreLog
	if wpOption == rmaweb.Manual {
		storeLogFlag = rmaweb.StoreLog
	}

	if err := uiHelper.RepairCompletedPageOperation(ctx, storeLogFlag); err != nil {
		s.Fatal("Fail to navigate to Repair Complete page: ", err)
	}

	if storeLogFlag == rmaweb.StoreLog {
		if err := uiHelper.VerifyLogIsSaved(ctx); err != nil {
			s.Fatal("Fail to verify that the log is saved in usb: ", err)
		}
	}
}

func generateActionCombinedToDisableWP(option rmaweb.WriteProtectDisableOption, enroll bool, destination rmaweb.DestinationOption, uiHelper *rmaweb.UIHelper) action.Action {

	if enroll && destination == rmaweb.DifferentUser && option == rmaweb.Rsu {
		return action.Combine("navigate to RSU page and turn off write protect",
			uiHelper.WelcomePageOperation,
			uiHelper.ComponentsPageOperation,
			uiHelper.OwnerPageOperation(destination),
			uiHelper.RSUPageOperation,
		)
	} else if !enroll && destination == rmaweb.DifferentUser && option == rmaweb.Rsu {
		return action.Combine("navigate to RSU page, choose different user and turn off write protect",
			uiHelper.WelcomePageOperation,
			uiHelper.ComponentsPageOperation,
			uiHelper.OwnerPageOperation(destination),
			uiHelper.WriteProtectPageChooseRSU,
			uiHelper.RSUPageOperation,
		)
	} else if !enroll && destination == rmaweb.SameUser && option == rmaweb.Rsu {
		return action.Combine("navigate to RSU page , choose same user and turn off write protect",
			uiHelper.WelcomePageOperation,
			uiHelper.ComponentsPageOperation,
			uiHelper.OwnerPageOperation(destination),
			uiHelper.WipeDevicePageOperation,
			uiHelper.WriteProtectPageChooseRSU,
			uiHelper.RSUPageOperation,
		)
	} else if !enroll && destination == rmaweb.SameUser && option == rmaweb.Manual {
		return action.Combine("navigate to Manual Disable Write Protect page, choose same user and turn off write protect",
			uiHelper.WelcomePageOperation,
			uiHelper.ComponentsPageOperation,
			uiHelper.OwnerPageOperation(destination),
			uiHelper.WipeDevicePageOperation,
			uiHelper.WriteProtectPageChooseManual,
		)
	}

	return nil
}
