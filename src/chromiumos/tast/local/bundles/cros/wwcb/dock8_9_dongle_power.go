// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// #8 USB Type-C multi-port dongle testing via a Dock

// "Pre-Condition:
// (Please note: Brand / Model number on test result)
// 1. Chromebook Device with USB Type-C support
// 2. External displays (Single /Dual)
// 3. Dock station /Hub /Dongle (i.e Multiport Type-C adapter)
// 4. Connection Type (HDMI/DP/VGA/DVI/USB-C on test result)
// 5. USB Peripherals (Flash drive, Mouse, Keyboard, Webcam, Headset)

// Testing scenarios:
// 1. Hotplug the [non-powered] dongle alone // another fixture for type-c (TPE prepare) after daq off
// 2. Power up the dongle (if support) // daq on
// 3. Hotplug peripheral(s) - one by one, or in combination - while dongle is plugged to Chromebook device // (TPE prepare fixture)
// 4. Unplug and plug-in back the dongle while all peripherals are plugged to the dongle // daq off then on, check peripherals using case 11 verification
// 4. Unplug, Flip, and plug-in back the dongle while all peripherals are plugged to the dongle // flip( James perpare) then check
// 5. Reboot the Chromebook device with peripherals connected to the powered dongle. // reboot then check
// 6. Remove power from dongle and reboot Chromebook device // daq off then reboot then check
// 7. Power up, Power down, and Power up again the dongle while all ports are busy with Ext-Display and HID (Keyboard, or Mouse) // daq on then check, daq off then on then check (moniter, keyboard, mouse)
// 8. SUSPEND/RESUME // 3 flows
// 8.1. Plug - Suspend - Resume . // daq on then sleep then wake up then check
// 8.2. Plug - Suspend - Unplug - Resume - Plug // daq on then sleep then daq off then wake up then daq on then check
// 8.3. Unplug - Suspend - Plug - Resume // daq on - sleep - daq of

// Procedure:
// Note: Cover all testing scenarios for the following conditions.

// 1)  Use the multi-port dongle(i.e. Apple adapter with HDMI and USB-A ports)  and with power source(high- and low- voltage adapter)
//  -- User should able to charge the Chromebook device using the powered dongle
//  -- charging(high or low voltage) icon present(max 3 seconds to appear) when charging, and disappear(max 3 seconds) when power is disconnected(normal/discharging icon present instead).
//  -- 'power_supply_info' output should be relevant.
//  -- Peripherals should be functioning as expected in both powered and non-powered dongle scenarios.

// 3) Check the peripherals discovery/mount/unmount times.
// -- Mount/Unmount should not take longer than acceptable time.

// 9) Convert the device into Tablet Mode and repeat the above steps in tablet mode.
//     Note: Onboard physical keyboard and Trackpad does not work in tablet mode.

// "

// Package wwcb contains local Tast tests that work with chromebook
package wwcb

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/wwcb/utils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Dock89DonglePower,
		Desc:         "USB Type-C multi-port dongle should work properly",
		Contacts:     []string{"allion-sw@allion.com"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      10 * time.Minute,
		Vars: []string{
			"WWCBIP",
			"DockingID",
			"1stExtDispID",
			"EthernetID",
			"AllUSBID"},
		Params: []testing.Param{
			{
				Val: false,
			},
			{
				Name: "tablet_mode",
				Val:  true,
			},
		},
	})
}

func Dock89DonglePower(ctx context.Context, s *testing.State) {
	tabletMode := s.Param().(bool)
	dockingID := s.RequiredVar("DockingID")
	extDispID1 := s.RequiredVar("1stExtDispID")
	ethernetID := s.RequiredVar("EthernetID")
	allUSBID := s.RequiredVar("AllUSBID")

	cr, err := chrome.New(ctx, chrome.GuestLogin())
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, tabletMode)
	if err != nil {
		s.Fatal("Failed to ensure clamshell/tablet mode: ", err)
	}
	defer cleanup(ctx)

	cleanupFixtures, err := utils.InitFixtures(ctx)
	if err != nil {
		s.Fatal("Failed to initialize fixtures: ", err)
	}
	defer cleanupFixtures(ctx)

	// Create usb controller.
	uc, err := utils.NewUsbController(ctx, dockingID, allUSBID)
	if err != nil {
		s.Fatal("Failed to create usb controller: ", err)
	}

	// Step 1 - Plug in dongle when dongle is non-powered.
	if err := dock89DonglePowerStep1(ctx, dockingID); err != nil {
		s.Fatal("Failed to execute step 1: ", err)
	}
	// step 2 - Power up dongle.
	if err := dock89DonglePowerStep2(ctx); err != nil {
		s.Fatal("Failed to execute step 2: ", err)
	}
	// Step 3 - Plug peripherals one by one, then verify.
	if err := dock89DonglePowerStep3(ctx, tconn, uc, extDispID1, ethernetID); err != nil {
		s.Fatal("Failed to execute step 3: ", err)
	}
	// Step 4 - Unplug & plug-in dongle then verify peripherals.
	if err := dock89DonglePowerStep4(ctx, tconn, uc, dockingID); err != nil {
		s.Fatal("Failed to execute step 4: ", err)
	}
	// Step 5 - Unplug, flip & plug-in dongle then verify peripherals.
	if err := dock89DonglePowerStep5(ctx, tconn, uc, dockingID); err != nil {
		s.Fatal("Failed to execute step 5: ", err)
	}
	// Step 6 - Reboot then verify peripherals.
	if err := dock89DonglePowerStep6(ctx, uc, tabletMode); err != nil {
		s.Fatal("Failed to execute step 6: ", err)
	}
	// Step 7 - Power down dongle & reboot, then verify peripherals.
	if err := dock89DonglePowerStep7(ctx, uc, tabletMode); err != nil {
		s.Fatal("Failed to execute step 7: ", err)
	}
	// Step 8 - Verify peripherals when power up / down dongle couple times.
	if err := dock89DonglePowerStep8(ctx, cr, uc, dockingID); err != nil {
		s.Fatal("Failed to execute step 8: ", err)
	}
	// Step 9 - Suspend and resume, then verify peripherals.
	if err := dock89DonglePowerStep9(ctx, cr, uc, dockingID); err != nil {
		s.Fatal("Failed to execute step 9: ", err)
	}
	// Step 10 - Plug in dongle, unplug dongle while chromebook is suspended, then verify peripherls.
	if err := dock89DonglePowerStep10(ctx, cr, uc, dockingID); err != nil {
		s.Fatal("Failed to execute step 10: ", err)
	}
	// Step 11 - Unplug dongle, plug in dongle while chromebook is suspended, then verify peripherals.
	if err := dock89DonglePowerStep11(ctx, cr, uc, dockingID); err != nil {
		s.Fatal("Failed to execute step 11: ", err)
	}
}

func dock89DonglePowerStep1(ctx context.Context, dockingID string) error {
	testing.ContextLog(ctx, "Step 1 - Plug in dongle when dongle is non-powered")
	if err := utils.ControlAviosys(ctx, "0", "1"); err != nil {
		return errors.Wrap(err, "failed to power down dongle")
	}
	if err := utils.SwitchFixture(ctx, dockingID, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to plug in dongle")
	}
	return nil
}

func dock89DonglePowerStep2(ctx context.Context) error {
	testing.ContextLog(ctx, "Step 2 - Power up dongle")
	if err := utils.ControlAviosys(ctx, "1", "1"); err != nil {
		return errors.Wrap(err, "failed to power down dongle")
	}
	if err := utils.VerifyPowerStatus(ctx, true); err != nil {
		return errors.Wrap(err, "failed to verify power is charing")
	}
	return nil
}

func dock89DonglePowerStep3(ctx context.Context, tconn *chrome.TestConn, uc *utils.UsbController, extDispID1, ethernetID string) error {
	testing.ContextLog(ctx, "Step 3 - Plug peripherals one by one, then verify peripherals")

	// Plug in ext-display.
	if err := utils.SwitchFixture(ctx, extDispID1, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to plug in ext-display")
	}
	if err := utils.VerifyExternalDisplay(ctx, tconn, true); err != nil {
		return errors.Wrap(err, "failed to verify external display is connected")
	}

	// Plug in ethernet.
	if err := utils.SwitchFixture(ctx, ethernetID, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to plug in ethernet")
	}
	if err := utils.VerifyEthernetStatus(ctx, true); err != nil {
		return errors.Wrap(err, "failed to verify ethernet is connected")
	}

	// Plug in usb devices.
	if err := uc.ControlUsbDevices(ctx, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to plug in usb devices")
	}
	if err := uc.VerifyUsbCount(ctx, true); err != nil {
		return errors.Wrap(err, "failed to verify usb devices count")
	}

	// Verify ext-audio is connected.
	if err := utils.VerifyExternalAudio(ctx, true); err != nil {
		return errors.Wrap(err, "failed to verfiy ext-audio is connected")
	}
	return nil
}

func dock89DonglePowerStep4(ctx context.Context, tconn *chrome.TestConn, uc *utils.UsbController, dockingID string) error {
	testing.ContextLog(ctx, "Step 4 - Unplug & plug-in dongle then verify peripherals")
	if err := utils.SwitchFixture(ctx, dockingID, "off", "0"); err != nil {
		return errors.Wrap(err, "failed to unplug dongle")
	}
	if err := utils.SwitchFixture(ctx, dockingID, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to plug in dongle")
	}
	if err := utils.VerifyPeripherals(ctx, tconn, uc, true); err != nil {
		return errors.Wrap(err, "failed to verify peripherals are connected")
	}
	return nil
}

func dock89DonglePowerStep5(ctx context.Context, tconn *chrome.TestConn, uc *utils.UsbController, dockingID string) error {
	testing.ContextLog(ctx, "Step 5 - Unplug, flip & plug in dongle then verify peripherals")
	if err := utils.SwitchFixture(ctx, dockingID, "off", "0"); err != nil {
		return errors.Wrap(err, "failed to unplug dongle")
	}
	if err := utils.SwitchFixture(ctx, dockingID, "flip", "0"); err != nil {
		return errors.Wrap(err, "failed to flip dongle")
	}
	if err := utils.VerifyPeripherals(ctx, tconn, uc, true); err != nil {
		return errors.Wrap(err, "failed to verify peripherals are connected")
	}
	return nil
}

func dock89DonglePowerStep6(ctx context.Context, uc *utils.UsbController, tabletMode bool) error {
	testing.ContextLog(ctx, "Step 6 - Reboot then verify peripherals")

	// Reboot Chromebook.
	cr, err := chrome.New(ctx, chrome.GuestLogin())
	if err != nil {
		return errors.Wrap(err, "failed to connect to Chrome")
	}
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create Test API connection")
	}

	// After reboot Chromebook, tablet mode would turn off.
	if err := ash.SetTabletModeEnabled(ctx, tconn, tabletMode); err != nil {
		return err
	}

	if err := utils.VerifyPeripherals(ctx, tconn, uc, true); err != nil {
		return errors.Wrap(err, "failed to verify peripherals are connected")
	}
	return nil
}

func dock89DonglePowerStep7(ctx context.Context, uc *utils.UsbController, tabletMode bool) error {
	testing.ContextLog(ctx, "Step 7 - Power down dongle & reboot, then verify peripherals")
	// Power down dongle.
	if err := utils.ControlAviosys(ctx, "0", "1"); err != nil {
		return errors.Wrap(err, "failed to power down dongle")
	}

	// Reboot Chromebook.
	cr, err := chrome.New(ctx, chrome.GuestLogin())
	if err != nil {
		return errors.Wrap(err, "failed to connect to Chrome")
	}
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create Test API connection")
	}

	// After reboot Chromebook, tablet mode would turn off.
	if err := ash.SetTabletModeEnabled(ctx, tconn, tabletMode); err != nil {
		return err
	}

	if err := utils.VerifyPeripherals(ctx, tconn, uc, false); err != nil {
		return errors.Wrap(err, "failed to verify peripherals are disconnected")
	}
	return nil
}

func dock89DonglePowerStep8(ctx context.Context, cr *chrome.Chrome, uc *utils.UsbController, dockingID string) error {
	testing.ContextLog(ctx, "Step 8 - Verify peripherals when power up / down couple times")

	if err := cr.Reconnect(ctx); err != nil {
		return errors.Wrap(err, "failed to reconnect to chromebook")
	}
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create Test API connection")
	}

	// Power up dongle then verify.
	if err := utils.ControlAviosys(ctx, "1", "1"); err != nil {
		return errors.Wrap(err, "failed to power up dongle")
	}
	// additional
	if err := utils.SwitchFixture(ctx, dockingID, "on", "0"); err != nil {
		return err
	}
	if err := utils.VerifyPeripherals(ctx, tconn, uc, true); err != nil {
		return errors.Wrap(err, "failed to verify peripherals are connected")
	}

	// Power down dongle then verify.
	if err := utils.ControlAviosys(ctx, "0", "1"); err != nil {
		return errors.Wrap(err, "failed to power down dongle")
	}
	if err := utils.VerifyPeripherals(ctx, tconn, uc, false); err != nil {
		return errors.Wrap(err, "failed to verify peripherals are disconnected")
	}

	// Power up dongle then verify.
	if err := utils.ControlAviosys(ctx, "1", "1"); err != nil {
		return errors.Wrap(err, "failed to power up dongle")
	}
	// additional
	if err := utils.SwitchFixture(ctx, dockingID, "on", "0"); err != nil {
		return err
	}
	if err := utils.VerifyPeripherals(ctx, tconn, uc, true); err != nil {
		return errors.Wrap(err, "failed to verify peripherals are connected")
	}
	return nil
}

func dock89DonglePowerStep9(ctx context.Context, cr *chrome.Chrome, uc *utils.UsbController, dockingID string) error {
	testing.ContextLog(ctx, "Step 9 - Suspend and resume, then verify peripherals are connected") // plug in
	if err := utils.SuspendAndResume(ctx, cr, 15); err != nil {
		return errors.Wrap(err, "failed to suspend the Chromebook")
	}
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create Test API connection")
	}
	if err := utils.VerifyPeripherals(ctx, tconn, uc, true); err != nil {
		return errors.Wrap(err, "failed to verify peripherals are connected")
	}
	return nil
}

func dock89DonglePowerStep10(ctx context.Context, cr *chrome.Chrome, uc *utils.UsbController, dockingID string) error {
	testing.ContextLog(ctx, "Step 10 - Unplug dongle while chromebook is suspended, plug dongle, then verify peripherls are connected")

	// Unplug dongle while suspend Chromebook.
	if err := utils.SwitchFixture(ctx, dockingID, "off", "5"); err != nil {
		return errors.Wrap(err, "failed to unplug dongle")
	}
	if err := utils.SuspendAndResume(ctx, cr, 15); err != nil {
		return errors.Wrap(err, "failed to suspend the Chromebook")
	}
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create Test API connection")
	}

	// After suspend and resume Chromebook, internet would disconnect.
	if err := shill.WaitForOnline(ctx); err != nil {
		return err
	}

	if err := utils.SwitchFixture(ctx, dockingID, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to plug in dongle")
	}

	if err := utils.VerifyPeripherals(ctx, tconn, uc, true); err != nil {
		return errors.Wrap(err, "failed to verify peripherals are connected")
	}
	return nil
}

func dock89DonglePowerStep11(ctx context.Context, cr *chrome.Chrome, uc *utils.UsbController, dockingID string) error {
	testing.ContextLog(ctx, "Step 11 - Unplug dongle, plug in dongle while chromebook is suspended, then verify peripherals are connected")

	if err := utils.SwitchFixture(ctx, dockingID, "off", "0"); err != nil {
		return errors.Wrap(err, "failed to unplug dongle")
	}

	// Plug in dongle while suspend Chromebook.
	if err := utils.SwitchFixture(ctx, dockingID, "on", "5"); err != nil {
		return errors.Wrap(err, "failed to plug in dongle")
	}
	if err := utils.SuspendAndResume(ctx, cr, 15); err != nil {
		return errors.Wrap(err, "failed to suspend the Chromebook")
	}
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create Test API connection")
	}

	// After suspend and resume Chromebook,, internet would lost.
	if err := shill.WaitForOnline(ctx); err != nil {
		return err
	}
	// additional
	if err := utils.SwitchFixture(ctx, dockingID, "on", "0"); err != nil {
		return err
	}

	if err := utils.VerifyPeripherals(ctx, tconn, uc, true); err != nil {
		return errors.Wrap(err, "failed to verify peripherals are connected")
	}
	return nil
}
