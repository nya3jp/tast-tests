// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

/***
#10 USB Type-C Docking station testing.
"
Pre-condition
(Please note: Brand / Model number on test result)
1. Chromebook Device with USB Type-C support
2. External displays (Single /Dual)
3. Dock station /Hub /Dongle (i.e Multiport Type-C adapter)
4. Connection Type (i.e. HDMI/DP/VGA/DVI/USB-C on test result)
5. USB Peripherals (Flash drive, Mouse, Keyboard, Webcam, Headset)
6. Ethernet connection.
7. 3.5mm headset.

Testing scenarios:
1. Boot the device with dock connected
2. Hotplug the dock alone
3. Hotplug the dock while all peripherals are plugged to the dock
4. Hotplug peripheral(s) - one by one, or in combination - while docking station is plugged to Chromebook device
5. Docking station only - Power up, Power down, and Up again the dock while all ports are busy with ExtDisplay and HID(kb, or mouse)
6. SUSPEND/RESUME
6.1. Plug - Suspend - Resume.
6.2. Plug - Suspend -Unplug -Resume - Plug
6.3. Unplug - Suspend - Plug - Resume

Procedure:
* Note: Cover all testing scenarios for the following steps.

1)  Use the multi-port docking station with peripheral ports used/plugged, with power source
 -- Users should be able to charge the Chromebook device using a powered dock station - charging icon present, 'power_supply_info' output.
 -- Peripherals should be functioning as expected

2) Confirm the docking station(i.e. HP Elite) as a power input. Use 'power_supply_info' to confirm power source.
 -- Users should be able to charge the Chromebook device using dock station.

3) Check the Docking station mount/unmount times on USB storage.
-- Mount/Unmount should not take longer than acceptable time.

4) Use the docking station to connect the external display.
-- Users should be able to use all the display ports from the docking station successfully.


5) Check the Ethernet port on the docking station.
-- Ethernet port should work and connect successfully.

6) Check all USB (2.0 and 3.0) ports available on the docking station.
-- All the ports should work properly for the tested peripherals.

7) Check 3.5mm port.
-- 3.5mm port should work properly with the audio peripheral.
[In most cases audio jack is presented as USB audio on Chromebook device audio settings/uber menu]

8) Make sure you cover all the available ports on the docking station.

9) Convert the device into Tablet Mode and repeat the above steps in tablet mode.
    Note: Onboard physical keyboard and trackpad does not work in tablet mode.
**/

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
		Func:         Dock1011DockingStation,
		Desc:         "USB Type-C Multi-Port adapter and Docking station should work properly",
		Contacts:     []string{"flin@google.com", "newmanliu19020@allion.corp-partner.google.com"},
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

func Dock1011DockingStation(ctx context.Context, s *testing.State) {
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
		s.Fatal("Failed to ensure in tablet mode: ", err)
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

	// Step 1 - Plug in dock, ext-display, ethernet.
	// Reboot Chromebook.
	// Verify power/ext-display/ethernet/audio are connected.
	if err := dock1011DockingStationStep1(ctx, dockingID, extDispID1, ethernetID); err != nil {
		s.Fatal("Failed to execute step 1: ", err)
	}
	if err := cr.Reconnect(ctx); err != nil {
		s.Fatal("Failed to reconnect to chromebook: ", err)
	}
	tconn, err = cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Step 2 - Unplug and plug in the dock.
	// Verify power/ext-display/ethernet/audio are connected.
	if err := dock1011DockingStationStep2(ctx, tconn, dockingID); err != nil {
		s.Fatal("Failed to execute step 2: ", err)
	}

	// Step 3 - Unplug dock, plug in usb devices, plug in the dock.
	// Verify peripherals are connected.
	if err := dock1011DockingStationStep3(ctx, tconn, uc, dockingID); err != nil {
		s.Fatal("Failed to execute step 3: ", err)
	}

	// Step 4 - Unplug dock and usb devices.
	// Plug in dock and usb devices.
	// Verify peripherals are connected.
	if err := dock1011DockingStationStep4(ctx, tconn, uc, dockingID); err != nil {
		s.Fatal("Failed to execute step 4: ", err)
	}

	// Step 5 - Power down and up dock
	// Verify peripherals are connected.
	if err := dock1011DockingStationStep5(ctx, tconn, uc); err != nil {
		s.Fatal("Failed to execute step 5: ", err)
	}

	// Step 6 - Suspend and resume.
	// Verify peripherals are connected.
	if err := dock1011DockingStationStep6(ctx, cr, uc, dockingID); err != nil {
		s.Fatal("Failed to execute step 6: ", err)
	}

	// Step 7 - Unplug dock while Chromebook is suspended.
	// Plug in dock, then verify peripherals are connected.
	if err := dock1011DockingStationStep7(ctx, cr, uc, dockingID); err != nil {
		s.Fatal("Failed to execute step 7: ", err)
	}

	// Step 8 - Unplug dock, plug dock while Chromebook is suspended.
	// Verify peripherals are connected.
	if err := dock1011DockingStationStep8(ctx, cr, uc, dockingID); err != nil {
		s.Fatal("Failed to execute step 8: ", err)
	}
}

func dock1011DockingStationStep1(ctx context.Context, dockingID, extDispID1, ethernetID string) error {
	testing.ContextLog(ctx, "Step 1 - Plug in dock, ext-display, ethernet, then reboot Chromebook, then verify")

	//Plug in dock, external display, ethernet.
	if err := utils.SwitchFixture(ctx, dockingID, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to plug in docking station")
	}
	if err := utils.SwitchFixture(ctx, extDispID1, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to plug in external display")
	}
	if err := utils.SwitchFixture(ctx, ethernetID, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to plug in ethernet")
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

	// Verify power, ext-diplay, ethernet,ext-audio are connected.
	if err := utils.VerifyPowerStatus(ctx, true); err != nil {
		return errors.Wrap(err, "failed to verify power is charging")
	}
	if err := utils.VerifyExternalDisplay(ctx, tconn, true); err != nil {
		return errors.Wrap(err, "failed to verify ext-display is connected")
	}
	if err := utils.VerifyEthernetStatus(ctx, true); err != nil {
		return errors.Wrap(err, "failed to verify ethernet is connected")
	}
	if err := utils.VerifyExternalAudio(ctx, true); err != nil {
		return errors.Wrap(err, "failed to verify ext-audio is connected")
	}
	return nil
}

func dock1011DockingStationStep2(ctx context.Context, tconn *chrome.TestConn, dockingID string) error {
	testing.ContextLog(ctx, "Step 2 - Unplug and plug in the dock, then verify")

	// Unplug dock.
	if err := utils.SwitchFixture(ctx, dockingID, "off", "0"); err != nil {
		return errors.Wrap(err, "failed to unplug docking station")
	}

	// Plug in dock.
	if err := utils.SwitchFixture(ctx, dockingID, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to plug in docking station")
	}

	// Verify power, ext-diplay, ethernet,ext-audio are connected.
	if err := utils.VerifyPowerStatus(ctx, true); err != nil {
		return errors.Wrap(err, "failed to verify power is charging")
	}
	if err := utils.VerifyExternalDisplay(ctx, tconn, true); err != nil {
		return errors.Wrap(err, "failed to verify ext-display is connected")
	}
	if err := utils.VerifyEthernetStatus(ctx, true); err != nil {
		return errors.Wrap(err, "failed to verify ethernet is connected")
	}
	if err := utils.VerifyExternalAudio(ctx, true); err != nil {
		return errors.Wrap(err, "failed to verify ext-audio is connected")
	}
	return nil
}
func dock1011DockingStationStep3(ctx context.Context, tconn *chrome.TestConn, uc *utils.UsbController, dockingID string) error {
	testing.ContextLog(ctx, "Step 3 - Unplug dock, plug in usb devices, plug in the dock then verify peripherals")

	// Unplug dock.
	if err := utils.SwitchFixture(ctx, dockingID, "off", "0"); err != nil {
		return errors.Wrap(err, "failed to unplug docking station")
	}

	// Plug in usb devices.
	if err := uc.ControlUsbDevices(ctx, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to control usb devices")
	}

	// Plug in dock.
	if err := utils.SwitchFixture(ctx, dockingID, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to plug in docking station")
	}

	// Verify peripherals are connected.
	if err := utils.VerifyPeripherals(ctx, tconn, uc, true); err != nil {
		return errors.Wrap(err, "failed to verify peripherals are connected")
	}
	return nil
}

func dock1011DockingStationStep4(ctx context.Context, tconn *chrome.TestConn, uc *utils.UsbController, dockingID string) error {
	testing.ContextLog(ctx, "Step 4 - Unplug dock and usb devices, plug in dock and usb devices, then verify peripherals")
	// Unplug dock.
	if err := utils.SwitchFixture(ctx, dockingID, "off", "0"); err != nil {
		return errors.Wrap(err, "failed to unplug docking station")
	}
	// Unplug usb devices.
	if err := uc.ControlUsbDevices(ctx, "off", "0"); err != nil {
		return errors.Wrap(err, "failed to unplug usb devices")
	}
	// Plug in dock.
	if err := utils.SwitchFixture(ctx, dockingID, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to plug in docking station")
	}
	// Plug in usb devices.
	if err := uc.ControlUsbDevices(ctx, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to plug in usb devices")
	}
	// Verify peripherals are connected.
	if err := utils.VerifyPeripherals(ctx, tconn, uc, true); err != nil {
		return errors.Wrap(err, "failed to verify peripherals are connected")
	}
	return nil
}

func dock1011DockingStationStep5(ctx context.Context, tconn *chrome.TestConn, uc *utils.UsbController) error {
	testing.ContextLog(ctx, "Step 5 - Power down and up dock, then verify peripherals are connected")
	// Power down docking.
	if err := utils.ControlAviosys(ctx, "0", "1"); err != nil {
		return errors.Wrap(err, "failed to power up docking station")
	}
	// Power up docking.
	if err := utils.ControlAviosys(ctx, "1", "1"); err != nil {
		return errors.Wrap(err, "failed to power up docking station")
	}
	// Verify peripherals are connected.
	if err := utils.VerifyPeripherals(ctx, tconn, uc, true); err != nil {
		return errors.Wrap(err, "failed to verify peripherals are connected")
	}
	return nil
}

func dock1011DockingStationStep6(ctx context.Context, cr *chrome.Chrome, uc *utils.UsbController, dockingID string) error {
	testing.ContextLog(ctx, "Suspend and resume, then verify peripherals are conneced")
	// Suspend and resume.
	if err := utils.SuspendAndResume(ctx, cr, 15); err != nil {
		return errors.Wrap(err, "failed to suspend the Chromebook")
	}
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create Test API connection")
	}
	// Verify peripherals are connected.
	if err := utils.VerifyPeripherals(ctx, tconn, uc, true); err != nil {
		return errors.Wrap(err, "failed to verify peripherals are connected")
	}
	return nil
}

func dock1011DockingStationStep7(ctx context.Context, cr *chrome.Chrome, uc *utils.UsbController, dockingID string) error {
	testing.ContextLog(ctx, "Unplug docking while Chromebook is suspended, then verify peripherals are connected")
	// Unplug docking while Chromebook is suspended.
	if err := utils.SwitchFixture(ctx, dockingID, "off", "5"); err != nil {
		return errors.Wrap(err, "failed to unplug docking station")
	}
	if err := utils.SuspendAndResume(ctx, cr, 15); err != nil {
		return errors.Wrap(err, "failed to suspend the Chromebook")
	}
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create Test API connection")
	}
	// When Chromebook is re-connected, need to wait for internet connectivity, otherwise would failed on http.Get.
	if err := shill.WaitForOnline(ctx); err != nil {
		return err
	}
	// Plug in docking.
	if err := utils.SwitchFixture(ctx, dockingID, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to plug in docking station")
	}
	// Verify peripherals are connected.
	if err := utils.VerifyPeripherals(ctx, tconn, uc, true); err != nil {
		return errors.Wrap(err, "failed to verify peripherals are connected")
	}
	return nil
}

func dock1011DockingStationStep8(ctx context.Context, cr *chrome.Chrome, uc *utils.UsbController, dockingID string) error {
	testing.ContextLog(ctx, "Unplug docking, plug in docking while Chromebook is suspended, then verify peripherals are connected")
	// Unplug docking.
	if err := utils.SwitchFixture(ctx, dockingID, "off", "0"); err != nil {
		return errors.Wrap(err, "failed to unplug docking station")
	}
	// Plug in docking while Chromebook is suspended.
	if err := utils.SwitchFixture(ctx, dockingID, "on", "5"); err != nil {
		return errors.Wrap(err, "failed to plug in docking station")
	}
	if err := utils.SuspendAndResume(ctx, cr, 15); err != nil {
		return errors.Wrap(err, "failed to suspend the Chromebook")
	}
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create Test API connection")
	}

	// After Chromebook suspend and resume, internet would lost.
	if err := shill.WaitForOnline(ctx); err != nil {
		return err
	}

	// additional
	if err := utils.SwitchFixture(ctx, dockingID, "on", "5"); err != nil {
		return errors.Wrap(err, "failed to plug in docking station")
	}

	// Verify peripherals are connected.
	if err := utils.VerifyPeripherals(ctx, tconn, uc, true); err != nil {
		return errors.Wrap(err, "failed to verify peripherals are connected")
	}
	return nil
}
