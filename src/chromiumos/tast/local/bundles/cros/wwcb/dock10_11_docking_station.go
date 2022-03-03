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
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/wwcb/utils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto/faillog"
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
	})
}

func Dock1011DockingStation(ctx context.Context, s *testing.State) {
	dockingID := s.RequiredVar("DockingID")
	extDispID1 := s.RequiredVar("1stExtDispID")
	ethernetID := s.RequiredVar("EthernetID")
	allUSBID := s.RequiredVar("AllUSBID")
	usbsID := strings.Split(allUSBID, ":")

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
	uc, err := utils.NewUsbController(ctx, dockingID, usbsID)
	if err != nil {
		s.Fatal("Failed to create usb recorder: ", err)
	}

	// 1. Boot the device with dock connected
	testing.ContextLog(ctx, "Step 1 - Boot the device with dock connected")

	// 2. Hotplug the dock alone
	if err := dock1011DockingStationStep2(ctx, dockingID); err != nil {
		s.Fatal("Failed to execute step 2: ", err)
	}

	// 3. Hotplug the dock while all peripherals are plugged to the dock
	// there is no conclusion to deal with for now

	// 4. Hotplug peripheral(s) - one by one, or in combination - while docking station is plugged to Chromebook device
	if err := dock1011DockingStationStep4(ctx, s, tconn, uc, extDispID1, ethernetID, usbsID); err != nil {
		s.Fatal("Failed to execute step 4: ", err)
	}

	// 5. Docking station only - Power up, Power down, and Up again the dock while all ports are busy with ExtDisplay and HID(kb, or mouse)
	if err := dock1011DockingStationStep5(ctx, s, tconn, uc); err != nil {
		s.Fatal("Failed to execute step 5: ", err)
	}

	// 6. Plug - Suspend - Resume.
	if err := dock1011DockingStationStep6(ctx, s, cr, uc, dockingID); err != nil {
		s.Fatal("Failed to execute step 6: ", err)
	}

	// 7. Plug - Suspend -Unplug -Resume - Plug
	if err := dock1011DockingStationStep7(ctx, s, cr, uc, dockingID); err != nil {
		s.Fatal("Failed to execute step 7: ", err)
	}

	// 8. Unplug - Suspend - Plug - Resume
	if err := dock1011DockingStationStep8(ctx, s, cr, uc, dockingID); err != nil {
		s.Fatal("Failed to execute step 8: ", err)
	}

	// // 9. into tablet, then repeat above steps
	// if err := dock1011DockingStationStep9(ctx, s, cr, uc, dockingID, extDispID1, ethernetID, peripheralID1, peripheralID2, peripheralID3, peripheralID4, peripheralID5); err != nil {
	// 	s.Fatal("Failed to execute step 9: ", err)
	// }
}

func dock1011DockingStationStep2(ctx context.Context, dockingID string) error {
	testing.ContextLog(ctx, "Step 2 - Hotplug the dock alone")
	if err := utils.SwitchFixture(ctx, dockingID, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to plug in docking station")
	}
	return nil
}

func dock1011DockingStationStep4(ctx context.Context, s *testing.State, tconn *chrome.TestConn, uc *utils.UsbController, extDispID1, ethernetID string, peripheralsID []string) error {
	testing.ContextLog(ctx, "Step 4 - plug peripherals one by one then check")
	// ext-display 1
	if err := utils.SwitchFixture(ctx, extDispID1, "on", "0"); err != nil {
		return err
	}
	// ethernet
	if err := utils.SwitchFixture(ctx, ethernetID, "on", "0"); err != nil {
		return err
	}
	// peripherals
	for _, pid := range peripheralsID {
		if err := utils.SwitchFixture(ctx, pid, "on", "0"); err != nil {
			return err
		}
	}
	// audio
	// check peripherals
	if err := utils.VerifyPeripherals(ctx, tconn, uc, true); err != nil {
		return errors.Wrap(err, "failed to check peripherals on docking station")
	}
	return nil
}

func dock1011DockingStationStep5(ctx context.Context, s *testing.State, tconn *chrome.TestConn, uc *utils.UsbController) error {
	testing.ContextLog(ctx, "Step 5 - check extDisp & HID after power up - down - up docking station")
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
	if err := utils.VerifyPeripherals(ctx, tconn, uc, true); err != nil {
		return errors.Wrap(err, "failed to check HID on docking station")
	}
	return nil
}

func dock1011DockingStationStep6(ctx context.Context, s *testing.State, cr *chrome.Chrome, uc *utils.UsbController, dockingID string) error {
	testing.ContextLog(ctx, "Plug in docking, suspend & wake up chromebook, then check peripherals on docking station")
	// plug in docking
	if err := utils.SwitchFixture(ctx, dockingID, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to plug in docking station")
	}
	// suspend & wake up chromebook
	tconn, err := utils.SuspendChromebook(ctx, cr)
	if err != nil {
		return errors.Wrap(err, "failed to suspend the wake up chromebook")
	}
	// check peripherals on docking station
	if err := utils.VerifyPeripherals(ctx, tconn, uc, true); err != nil {
		return errors.Wrap(err, "failed to check peripherals on docking station")
	}
	return nil
}

func dock1011DockingStationStep7(ctx context.Context, s *testing.State, cr *chrome.Chrome, uc *utils.UsbController, dockingID string) error {
	testing.ContextLog(ctx, "Plug in docking, suspend chromebook, unplug docking, wake up it, then check peripherals on docking station")
	// plug in station
	if err := utils.SwitchFixture(ctx, dockingID, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to plug in docking station")
	}
	// unplug station later
	if err := utils.SwitchFixture(ctx, dockingID, "off", "5"); err != nil {
		return errors.Wrap(err, "failed to unplug docking station")
	}
	// suspend then resume
	tconn, err := utils.SuspendChromebook(ctx, cr)
	if err != nil {
		return errors.Wrap(err, "failed to suspend chromebook")
	}
	// plug in docking
	if err := utils.SwitchFixture(ctx, dockingID, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to plug in docking station")
	}
	// check peripherals
	if err := utils.VerifyPeripherals(ctx, tconn, uc, true); err != nil {
		return errors.Wrap(err, "failed to check peripherals on docking station")
	}
	return nil
}

func dock1011DockingStationStep8(ctx context.Context, s *testing.State, cr *chrome.Chrome, uc *utils.UsbController, dockingID string) error {
	testing.ContextLog(ctx, "Unplug docking, suspend chromebook, plug in docking , wake up it, then check peripherals on docking station")
	// unplug station
	if err := utils.SwitchFixture(ctx, dockingID, "off", "0"); err != nil {
		return errors.Wrap(err, "failed to unplug docking station")
	}
	// plug in station later
	if err := utils.SwitchFixture(ctx, dockingID, "on", "5"); err != nil {
		return errors.Wrap(err, "failed to plug in docking station")
	}
	// suspend - resume
	tconn, err := utils.SuspendChromebook(ctx, cr)
	if err != nil {
		return errors.Wrap(err, "failed to suspend then reconnect chromebook")
	}
	// check peripherals
	if err := utils.VerifyPeripherals(ctx, tconn, uc, false); err != nil {
		return errors.Wrap(err, "failed to check peripherals on docking station")
	}
	return nil
}

func dock1011DockingStationStep9(ctx context.Context, s *testing.State, cr *chrome.Chrome, uc *utils.UsbController, dockingID, extDispID1, ethernetID, peripheralID1, peripheralID2, peripheralID3, peripheralID4, peripheralID5 string) error {
	testing.ContextLog(ctx, "Step 9 - Enable tablet mode, then repeat above steps")

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
		return errors.Wrap(err, "failed to ensure in tablet mode")
	}
	defer cleanup(ctx)

	// Note: Onboard physical keyboard and trackpad does not work in tablet mode.

	// repeat above steps
	// 2. Hotplug the dock alone
	if err := dock1011DockingStationStep2(ctx, dockingID); err != nil {
		s.Fatal("Failed to execute step 2: ", err)
	}

	// 3. Hotplug the dock while all peripherals are plugged to the dock

	// 4. Hotplug peripheral(s) - one by one, or in combination - while docking station is plugged to Chromebook device
	// if err := dock1011DockingStationStep4(ctx, s, tconn, uc, extDispID1, ethernetID, peripheralID1, peripheralID2, peripheralID3, peripheralID4, peripheralID5); err != nil {
	// 	s.Fatal("Failed to execute step 4: ", err)
	// }
	// 5. Docking station only - Power up, Power down, and Up again the dock while all ports are busy with ExtDisplay and HID(kb, or mouse)
	if err := dock1011DockingStationStep5(ctx, s, tconn, uc); err != nil {
		s.Fatal("Failed to execute step 5: ", err)
	}
	// 6. Plug - Suspend - Resume.
	if err := dock1011DockingStationStep6(ctx, s, cr, uc, dockingID); err != nil {
		s.Fatal("Failed to execute step 6: ", err)
	}
	// 7. Plug - Suspend -Unplug -Resume - Plug
	if err := dock1011DockingStationStep7(ctx, s, cr, uc, dockingID); err != nil {
		s.Fatal("Failed to execute step 7: ", err)
	}
	// 8. Unplug - Suspend - Plug - Resume
	if err := dock1011DockingStationStep8(ctx, s, cr, uc, dockingID); err != nil {
		s.Fatal("Failed to execute step 8: ", err)
	}
	return nil
}
