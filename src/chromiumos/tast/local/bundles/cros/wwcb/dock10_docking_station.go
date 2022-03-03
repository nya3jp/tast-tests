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
"
**/

// "Test step:
// 1. Power the Chromebook On.
// 2. Sign-in account.
// 3. Connect external monitor to the docking station or hub (Manual)
// 4. Run verification 1.
// 5. Mount/Unmounted USB storage(USB hub power on/off )
// 6. Run verification 2.3.4.5.6
// 7. Enable tablet mode repeat Step 4~6"

// Automation verification
// 1. check "" power_supply_info"" output
// 2. check unmount until mount times(移除設備後到系統再次讀取到設備的時間差)
// 3. Check external monitor properly
// 4. check Ethernet has been connected
// 5. check usb device info
// 6. check 3.5mm device info"

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
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Dock10DockingStation,
		Desc:         "USB Type-C Multi-Port adapter and Docking station should work properly",
		Contacts:     []string{"allion-sw@allion.com"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Timeout:      10 * time.Minute,
		Pre:          chrome.LoggedIn(), //Boot-up and Sign-In to the device
		Vars:         utils.InputArguments,
	})
}

func Dock10DockingStation(ctx context.Context, s *testing.State) {

	// set up
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to open connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	if err := utils.InitFixtures(ctx); err != nil {
		s.Fatal("Failed to initialize fixtures: ", err)
	}

	// build usb recorder
	uc, err := utils.NewUsbController(ctx, s)
	if err != nil {
		s.Fatal("Failed to create usb recorder: ", err)
	}

	// 1. Boot the device with dock connected
	s.Log("Step 1 - Boot the device with dock connected ")

	// 2. Hotplug the dock alone
	if err := dock10DockingStationStep2(ctx, s); err != nil {
		s.Fatal("Failed to execute step 2: ", err)
	}

	// 3. Hotplug the dock while all peripherals are plugged to the dock
	// there is no conclusion to deal with for now

	// 4. Hotplug peripheral(s) - one by one, or in combination - while docking station is plugged to Chromebook device
	if err := dock10DockingStationStep4(ctx, s, tconn, uc); err != nil {
		s.Fatal("Failed to execute step 4: ", err)
	}

	// 5. Docking station only - Power up, Power down, and Up again the dock while all ports are busy with ExtDisplay and HID(kb, or mouse)
	if err := dock10DockingStationStep5(ctx, s, tconn, uc); err != nil {
		s.Fatal("Failed to execute step 5: ", err)
	}

	// 6. Plug - Suspend - Resume.
	if err := dock10DockingStationStep6(ctx, s, cr, uc); err != nil {
		s.Fatal("Failed to execute step 6: ", err)
	}

	// 7. Plug - Suspend -Unplug -Resume - Plug
	if err := dock10DockingStationStep7(ctx, s, cr, uc); err != nil {
		s.Fatal("Failed to execute step 7: ", err)
	}

	// 8. Unplug - Suspend - Plug - Resume
	if err := dock10DockingStationStep8(ctx, s, cr, uc); err != nil {
		s.Fatal("Failed to execute step 8: ", err)
	}

	// 9. into tablet, then repeat above steps
	if err := dock10DockingStationStep9(ctx, s, cr, uc); err != nil {
		s.Fatal("Failed to execute step 9: ", err)
	}
}

func dock10DockingStationStep2(ctx context.Context, s *testing.State) error {

	s.Log("Step 2 - Hotplug the dock alone")

	if err := utils.ControlFixture(ctx, s, utils.StationType, utils.StationIndex, utils.ActionPlugin, false); err != nil {
		return errors.Wrap(err, "failed to plug in docking station")
	}

	return nil
}

func dock10DockingStationStep4(ctx context.Context, s *testing.State, tconn *chrome.TestConn, uc *utils.UsbController) error {

	s.Log("Step 4 - plug peripherals one by one then check ")

	// plug peripherals one by one
	if err := utils.ControlPeripherals(ctx, s, uc, utils.ActionPlugin, false); err != nil {
		return errors.Wrap(err, "failed to plug in peripherals to docking station one by one")
	}

	// check peripherals
	if err := utils.VerifyPeripherals(ctx, s, tconn, uc, utils.IsConnect); err != nil {
		return errors.Wrap(err, "failed to check peripherals on docking station")
	}

	return nil
}

func dock10DockingStationStep5(ctx context.Context, s *testing.State, tconn *chrome.TestConn, uc *utils.UsbController) error {

	s.Log("Step 5 - check extDisp & HID after power up - down - up docking station")

	// power up docking
	if err := utils.SetStationPower(ctx, utils.PSUPowerOn); err != nil {
		return errors.Wrap(err, "failed to power up docking station")
	}

	// power off docking
	if err := utils.SetStationPower(ctx, utils.PSUPowerOff); err != nil {
		return errors.Wrap(err, "failed to power off docking station")
	}

	// power up docking
	if err := utils.SetStationPower(ctx, utils.PSUPowerOn); err != nil {
		return errors.Wrap(err, "failed to power up docking station")
	}

	// check peripherals on station
	if err := utils.VerifyPeripherals(ctx, s, tconn, uc, utils.IsConnect); err != nil {
		return errors.Wrap(err, "failed to check HID on docking station")
	}

	return nil
}

func dock10DockingStationStep6(ctx context.Context, s *testing.State, cr *chrome.Chrome, uc *utils.UsbController) error {

	s.Log("Plug in docking, suspend & wake up chromebook, then check peripherals on docking station")

	// plug in docking
	if err := utils.ControlFixture(ctx, s, utils.StationType, utils.StationIndex, utils.ActionPlugin, false); err != nil {
		return errors.Wrap(err, "failed to plug-in docking station")
	}

	// suspend & wake up chromebook
	tconn, err := utils.SuspendChromebook(ctx, s, cr)
	if err != nil {
		return errors.Wrap(err, "failed to suspend the wake up chromebook")
	}

	// check peripherals on docking station
	if err := utils.VerifyPeripherals(ctx, s, tconn, uc, utils.IsConnect); err != nil {
		return errors.Wrap(err, "failed to check peripherals on docking station")
	}

	return nil
}

func dock10DockingStationStep7(ctx context.Context, s *testing.State, cr *chrome.Chrome, uc *utils.UsbController) error {

	s.Log("Plug in docking, suspend chromebook, unplug docking, wake up it, then check peripherals on docking station")

	// plug in station

	if err := utils.ControlFixture(ctx, s, utils.StationType, utils.StationIndex, utils.ActionPlugin, false); err != nil {
		return errors.Wrap(err, "failed to plug-in docking station")
	}

	// unplug station later
	if err := utils.ControlFixture(ctx, s, utils.StationType, utils.StationIndex, utils.ActionUnplug, true); err != nil {
		return errors.Wrap(err, "failed to unplug docking station later")
	}

	// suspend then resume
	tconn, err := utils.SuspendChromebook(ctx, s, cr)
	if err != nil {
		return errors.Wrap(err, "failed to suspend chromebook")
	}

	// plug in docking
	if err := utils.ControlFixture(ctx, s, utils.StationType, utils.StationIndex, utils.ActionPlugin, false); err != nil {
		return errors.Wrap(err, "failed to plug-in docking station")
	}

	// check peripherals
	if err := utils.VerifyPeripherals(ctx, s, tconn, uc, utils.IsConnect); err != nil {
		return errors.Wrap(err, "failed to check peripherals on docking station")
	}

	return nil
}

func dock10DockingStationStep8(ctx context.Context, s *testing.State, cr *chrome.Chrome, uc *utils.UsbController) error {

	s.Log("Unplug docking, suspend chromebook, plug in docking , wake up it, then check peripherals on docking station")

	// unplug station
	if err := utils.ControlFixture(ctx, s, utils.StationType, utils.StationIndex, utils.ActionUnplug, false); err != nil {
		return errors.Wrap(err, "failed to connect docking station to chromebook")
	}

	// plug in station later
	if err := utils.ControlFixture(ctx, s, utils.StationType, utils.StationIndex, utils.ActionPlugin, true); err != nil {
		return errors.Wrap(err, "failed to plug in docking station later")
	}

	// suspend - resume
	tconn, err := utils.SuspendChromebook(ctx, s, cr)
	if err != nil {
		return errors.Wrap(err, "failed to suspend then reconnect chromebook")
	}

	// check peripherals
	if err := utils.VerifyPeripherals(ctx, s, tconn, uc, utils.IsDisconnect); err != nil {
		return errors.Wrap(err, "failed to check peripherals on docking station")
	}

	return nil
}

func dock10DockingStationStep9(ctx context.Context, s *testing.State, cr *chrome.Chrome, uc *utils.UsbController) error {

	s.Log("Step 9 - Enable tablet mode, then repeat above steps ")

	// reconnect chromebook
	if err := cr.Reconnect(ctx); err != nil {
		return errors.Wrap(err, "failed to reconnect to chromebook")
	}

	// get test connection
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create Test API connection")
	}

	// ensure tablet mode is enabled
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
	if err != nil {
		return errors.Wrap(err, "failed to ensure in tablet mode ")
	}
	defer cleanup(ctx)

	// Note: Onboard physical keyboard and trackpad does not work in tablet mode.

	// repeat above steps
	// 2. Hotplug the dock alone
	if err := dock10DockingStationStep2(ctx, s); err != nil {
		s.Fatal("Failed to execute step 2: ", err)
	}

	// 3. Hotplug the dock while all peripherals are plugged to the dock

	// 4. Hotplug peripheral(s) - one by one, or in combination - while docking station is plugged to Chromebook device
	if err := dock10DockingStationStep4(ctx, s, tconn, uc); err != nil {
		s.Fatal("Failed to execute step 4: ", err)
	}

	// 5. Docking station only - Power up, Power down, and Up again the dock while all ports are busy with ExtDisplay and HID(kb, or mouse)
	if err := dock10DockingStationStep5(ctx, s, tconn, uc); err != nil {
		s.Fatal("Failed to execute step 5: ", err)
	}

	// 6. Plug - Suspend - Resume.
	if err := dock10DockingStationStep6(ctx, s, cr, uc); err != nil {
		s.Fatal("Failed to execute step 6: ", err)
	}

	// 7. Plug - Suspend -Unplug -Resume - Plug
	if err := dock10DockingStationStep7(ctx, s, cr, uc); err != nil {
		s.Fatal("Failed to execute step 7: ", err)
	}

	// 8. Unplug - Suspend - Plug - Resume
	if err := dock10DockingStationStep8(ctx, s, cr, uc); err != nil {
		s.Fatal("Failed to execute step 8: ", err)
	}

	return nil
}
