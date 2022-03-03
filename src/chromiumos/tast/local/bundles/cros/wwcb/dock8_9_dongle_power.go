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
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/wwcb/utils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto/faillog"
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
			"AllPeripheralID"},
	})
}

func Dock89DonglePower(ctx context.Context, s *testing.State) {
	dockingID := s.RequiredVar("DockingID")
	extDispID1 := s.RequiredVar("1stExtDispID")
	ethernetID := s.RequiredVar("EthernetID")
	allPeripheralID := s.RequiredVar("AllPeripheralID")
	peripheralsID := strings.Split(allPeripheralID, ":")

	// connect to chrome
	cr, err := chrome.New(ctx, chrome.ARCEnabled())
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	// create api connection
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// cras, err := audio.NewCras(ctx)
	// if err != nil {
	// 	// return errors.Wrap(err, "failed to create cras")
	// 	s.Fatal(err)
	// }
	// nodes, err := cras.GetNodes(ctx)
	// if err != nil {
	// 	// return errors.Wrap(err, "failed to get nodes from cras")
	// 	s.Fatal(err)
	// }
	// utils.PrettyPrint(ctx, nodes)

	if err := utils.InitFixtures(ctx); err != nil {
		s.Fatal("Failed to initialize fixtures: ", err)
	}

	// build usb recorder
	uc, err := utils.NewUsbController(ctx, dockingID, peripheralsID)
	if err != nil {
		s.Fatal("Failed to create usb recorder: ", err)
	}

	// step 1 - connect dongle when dongle is non-powered
	if err := dock89DonglePowerStep1(ctx, dockingID); err != nil {
		s.Fatal("Failed to execute step 1: ", err)
	}
	// step 2 - power up dongle
	if err := dock89DonglePowerStep2(ctx); err != nil {
		s.Fatal("Failed to execute step 2: ", err)
	}
	// Step 3 - plug peripherals one by one, then check
	if err := dock89DonglePowerStep3(ctx, tconn, uc, extDispID1, ethernetID, peripheralsID); err != nil {
		s.Fatal("Failed to execute step 3: ", err)
	}
	// // Step 4 - unplug & plug-in dongle then check all peripherals
	// if err := dock89DonglePowerStep4(ctx, tconn, uc, dockingID); err != nil {
	// 	s.Fatal("Failed to execute step 4: ", err)
	// }
	// // Step 5 - unplug, flip & plug-in dongle then check all peripherals
	// if err := dock89DonglePowerStep5(ctx, tconn, uc, dockingID); err != nil {
	// 	s.Fatal("Failed to execute step 5: ", err)
	// }
	// // Step 6 - reboot then check peripherals
	// if err := dock89DonglePowerStep6(ctx, false, uc); err != nil {
	// 	s.Fatal("Failed to execute step 6: ", err)
	// }
	// // Step 7 - power off dongle & reboot, then check peripherals
	// if err := dock89DonglePowerStep7(ctx, false, uc); err != nil {
	// 	s.Fatal("Failed to execute step 7: ", err)
	// }
	// Step 8 - check peripherals when power up / down couple times
	if err := dock89DonglePowerStep8(ctx, cr, uc, dockingID); err != nil {
		s.Fatal("Failed to execute step 8: ", err)
	}
	// Step 9 - plug in dongle, sleep chromebook & wake up it, then check peripherals
	if err := dock89DonglePowerStep9(ctx, cr, uc, dockingID); err != nil {
		s.Fatal("Failed to execute step 9: ", err)
	}
	// Step 10 - plug in dongle, suspend chromebook, unplug dongle, wake up it, plug dongle, then check peripherls
	if err := dock89DonglePowerStep10(ctx, cr, uc, dockingID); err != nil {
		s.Fatal("Failed to execute step 10: ", err)
	}
	// Step 11 - unplug dongle, suspend chromebook, plug in dongle, wake up it, then check peripherals
	if err := dock89DonglePowerStep11(ctx, cr, uc, dockingID); err != nil {
		s.Fatal("Failed to execute step 11: ", err)
	}
	// // Step 12 - into tablet, then repeat above steps
	// if err := dock89DonglePowerStep12(ctx, cr, uc, dockingID, extDispID1, ethernetID, usbsID); err != nil {
	// 	s.Fatal("Failed to execute step 12: ", err)
	// }
}

func dock89DonglePowerStep1(ctx context.Context, dockingID string) error {
	testing.ContextLog(ctx, "Step 1 - Connect dongle when dongle is non-powered")
	// power off dongle
	if err := utils.SetStationPower(ctx, utils.PSUPowerOff); err != nil {
		return errors.Wrap(err, "failed to power off dongle")
	}
	// plug in dongle
	if err := utils.SwitchFixture(ctx, dockingID, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to connect dongle")
	}
	return nil
}

func dock89DonglePowerStep2(ctx context.Context) error {
	testing.ContextLog(ctx, "Step 2 - Power up dongle")
	if err := utils.SetStationPower(ctx, utils.PSUPowerOn); err != nil {
		return errors.Wrap(err, "failed to power on dongle")
	}
	// verify power
	if err := utils.VerifyPowerStatus(ctx, utils.IsConnect); err != nil {
		return err
	}
	return nil
}

func dock89DonglePowerStep3(ctx context.Context, tconn *chrome.TestConn, uc *utils.UsbController, extDispID1, ethernetID string, peripheralsID []string) error {
	testing.ContextLog(ctx, "Step 3 - Plug peripherals one by one, then check")
	// ext-display 1
	if err := utils.SwitchFixture(ctx, extDispID1, "on", "0"); err != nil {
		return err
	}
	if err := utils.VerifyExternalDisplay(ctx, tconn, utils.IsConnect); err != nil {
		return err
	}

	// ethernet
	if err := utils.SwitchFixture(ctx, ethernetID, "on", "0"); err != nil {
		return err
	}

	if err := utils.VerifyEthernetStatus(ctx, utils.IsConnect); err != nil {
		return err
	}

	// peripherals
	for _, pid := range peripheralsID {
		if err := utils.SwitchFixture(ctx, pid, "on", "0"); err != nil {
			return err
		}
	}
	if err := uc.VerifyUsbCount(ctx, utils.IsConnect); err != nil {
		return err
	}

	// audio
	// verify external audio
	if err := utils.VerifyExternalAudio(ctx, utils.IsConnect); err != nil {
		return err
	}
	return nil
}

func dock89DonglePowerStep4(ctx context.Context, tconn *chrome.TestConn, uc *utils.UsbController, dockingID string) error {
	testing.ContextLog(ctx, "Step 4 - Unplug & plug-in dongle then check all peripherals")
	// unplug dongle
	if err := utils.SwitchFixture(ctx, dockingID, "off", "0"); err != nil {
		return errors.Wrap(err, "failed to unplug dongle")
	}
	// plug-in dongle
	if err := utils.SwitchFixture(ctx, dockingID, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to plug in dongle")
	}
	// check peripherals
	if err := utils.VerifyPeripherals(ctx, tconn, uc, utils.IsConnect); err != nil {
		return errors.Wrap(err, "failed to check peripherals on dongle")
	}
	return nil
}

func dock89DonglePowerStep5(ctx context.Context, tconn *chrome.TestConn, uc *utils.UsbController, dockingID string) error {
	testing.ContextLog(ctx, "Step 5 - Unplug, flip & plug in dongle then check all peripherals")
	// unplug dongle
	if err := utils.SwitchFixture(ctx, dockingID, "off", "0"); err != nil {
		return errors.Wrap(err, "failed to unplug dongle")
	}
	// flip then plug in dongle
	if err := utils.SwitchFixture(ctx, dockingID, "flip", "0"); err != nil {
		return errors.Wrap(err, "failed to flip dongle")
	}
	// check peripherals
	if err := utils.VerifyPeripherals(ctx, tconn, uc, utils.IsConnect); err != nil {
		return errors.Wrap(err, "failed to check peripherals on dongle")
	}
	return nil
}

func dock89DonglePowerStep6(ctx context.Context, intoTablet bool, uc *utils.UsbController) error {
	testing.ContextLog(ctx, "Step 6 - Reboot then check peripherals")
	cr, err := chrome.New(ctx, chrome.ARCEnabled())
	if err != nil {
		return errors.Wrap(err, "failed to connect to Chrome")
	}
	defer func() {
		if err := cr.Close(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to close Chrome while (re)booting ARC: ", err)
		}
	}()
	// re-create api connection
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create Test API connection")
	}
	// into tablet mode,  mode change back cuz reboot
	if intoTablet == true {
		_, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
		if err != nil {
			return errors.Wrap(err, "failed to ensure in tablet mode")
		}
		// defer cleanup(ctx)
	}
	// check peripherals
	if err := utils.VerifyPeripherals(ctx, tconn, uc, utils.IsConnect); err != nil {
		return errors.Wrap(err, "failed to check peripherals on dongle")
	}
	return nil
}

func dock89DonglePowerStep7(ctx context.Context, intoTablet bool, uc *utils.UsbController) error {
	testing.ContextLog(ctx, "Step 7 - Power off dongle & reboot, then check peripherals")
	// power off dongle
	if err := utils.SetStationPower(ctx, utils.PSUPowerOff); err != nil {
		return errors.Wrap(err, "failed to power off dongle")
	}
	// reboot
	cr, err := chrome.New(ctx, chrome.ARCEnabled())
	if err != nil {
		return errors.Wrap(err, "failed to connect to Chrome")
	}
	defer func() {
		if err := cr.Close(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to close Chrome while (re)booting ARC: ", err)
		}
	}()
	// re-create api connection
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create Test API connection")
	}
	if intoTablet == true {
		_, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
		if err != nil {
			return errors.Wrap(err, "failed to ensure in tablet mode")
		}
		// defer cleanup(ctx)
	}
	// check peripherals is not on dongle
	if err := utils.VerifyPeripherals(ctx, tconn, uc, utils.IsDisconnect); err != nil {
		return errors.Wrap(err, "failed to check peripherals is not on dongle")
	}
	return nil
}

func dock89DonglePowerStep8(ctx context.Context, cr *chrome.Chrome, uc *utils.UsbController, dockingID string) error {
	testing.ContextLog(ctx, "Step 8 - Check peripherals when power up / down couple times")

	if err := cr.Reconnect(ctx); err != nil {
		return errors.Wrap(err, "failed to reconnect to chromebook")
	}
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create Test API connection")
	}

	// power up
	if err := utils.SetStationPower(ctx, utils.PSUPowerOn); err != nil {
		return errors.Wrap(err, "failed to power on dongle")
	}
	// check peripherals are connected
	if err := utils.VerifyPeripherals(ctx, tconn, uc, utils.IsConnect); err != nil {
		return errors.Wrap(err, "failed to check peripherals on dongle")
	}
	// power down
	if err := utils.SetStationPower(ctx, utils.PSUPowerOff); err != nil {
		return errors.Wrap(err, "failed to power off dongle")
	}
	// check peripherals are disconnected
	if err := utils.VerifyPeripherals(ctx, tconn, uc, utils.IsDisconnect); err != nil {
		return errors.Wrap(err, "failed to check peripherals on dongle")
	}
	// power up
	if err := utils.SetStationPower(ctx, utils.PSUPowerOn); err != nil {
		return errors.Wrap(err, "failed to power on dongle")
	}
	if err := utils.SwitchFixture(ctx, dockingID, "on", "0"); err != nil {
		return err
	}
	// check peripherals are connected
	if err := utils.VerifyPeripherals(ctx, tconn, uc, utils.IsConnect); err != nil {
		return errors.Wrap(err, "failed to check peripherals on dongle")
	}
	return nil
}

func dock89DonglePowerStep9(ctx context.Context, cr *chrome.Chrome, uc *utils.UsbController, dockingID string) error {
	testing.ContextLog(ctx, "Step 9 - Suspend Chromebook for 15s, then check peripherals") // plug in
	// suspend then reconnect chromebook
	tconn, err := utils.SuspendChromebook(ctx, cr)
	if err != nil {
		return errors.Wrap(err, "failed to suspend then reconnect chromebook")
	}
	// check peripherals
	if err := utils.VerifyPeripherals(ctx, tconn, uc, utils.IsConnect); err != nil {
		return errors.Wrap(err, "failed to check peripherals on dongle")
	}
	return nil
}

func dock89DonglePowerStep10(ctx context.Context, cr *chrome.Chrome, uc *utils.UsbController, dockingID string) error {
	testing.ContextLog(ctx, "Step 10 - Plug in dongle, suspend chromebook for 15s, unplug dongle, wake up it, plug dongle, then check peripherls")
	// unplug dongle later
	if err := utils.SwitchFixture(ctx, dockingID, "off", "5"); err != nil {
		return errors.Wrap(err, "failed to unplug dongle")
	}
	// suspend then wake up chromebook
	tconn, err := utils.SuspendChromebook(ctx, cr)
	if err != nil {
		return errors.Wrap(err, "failed to suspend then reconnect chromebook")
	}
	// plug in dongle
	if err := utils.SwitchFixture(ctx, dockingID, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to plug in dongle")
	}
	// check peripherals
	if err := utils.VerifyPeripherals(ctx, tconn, uc, utils.IsConnect); err != nil {
		return errors.Wrap(err, "failed to check peripherals on dongle")
	}
	return nil
}

func dock89DonglePowerStep11(ctx context.Context, cr *chrome.Chrome, uc *utils.UsbController, dockingID string) error {
	testing.ContextLog(ctx, "Step 11 - Unplug dongle, suspend chromebook for 15s, plug in dongle, wake up it, then check peripherals")
	// unplug dongle
	if err := utils.SwitchFixture(ctx, dockingID, "off", "0"); err != nil {
		return errors.Wrap(err, "failed to unplug dongle")
	}
	// plug in dongle later
	if err := utils.SwitchFixture(ctx, dockingID, "on", "5"); err != nil {
		return errors.Wrap(err, "failed to plug in dongle")
	}
	// suspend then wake up chromebook
	tconn, err := utils.SuspendChromebook(ctx, cr)
	if err != nil {
		return errors.Wrap(err, "failed to suspend then reconnect chromebook")
	}
	// check peripherals
	if err := utils.VerifyPeripherals(ctx, tconn, uc, utils.IsConnect); err != nil {
		return errors.Wrap(err, "failed to check peripherals on dongle")
	}
	return nil
}

// func dock89DonglePowerStep12(ctx context.Context, cr *chrome.Chrome, uc *utils.UsbController, dockingID, extDispID1, ethernetID string, usbsID []string) error {
// 	testing.ContextLog(ctx, "Step 12 - into tablet mode, repeat above steps")

// 	if err := cr.Reconnect(ctx); err != nil {
// 		return errors.Wrap(err, "failed to reconnect to chromebook")
// 	}
// 	tconn, err := cr.TestAPIConn(ctx)
// 	if err != nil {
// 		return errors.Wrap(err, "failed to create Test API connection")
// 	}
// 	// into tablet mode, repeat above steps
// 	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
// 	if err != nil {
// 		s.Fatal("Failed to ensure in tablet mode: ", err)
// 	}
// 	defer cleanup(ctx)

// 	// repeat above steps

// 	// step 1 - connect dongle when dongle is non-powered
// 	if err := dock89DonglePowerStep1(ctx, dockingID); err != nil {
// 		return errors.Wrap(err,"failed to execute step 1")
// 	}
// 	// step 2 - power up dongle
// 	if err := dock89DonglePowerStep2(ctx); err != nil {
// 		return errors.Wrap(err,"failed to execute step 2")
// 	}
// 	// Step 3 - plug peripherals one by one, then check
// 	if err := dock89DonglePowerStep3(ctx, tconn, uc, extDispID1, ethernetID, usbsID); err != nil {
// 		return errors.Wrap(err,"failed to execute step 3")
// 	}
// 	// Step 4 - unplug & plug-in dongle then check all peripherals
// 	if err := dock89DonglePowerStep4(ctx, tconn, uc, dockingID); err != nil {
// 		return errors.Wrap(err,"failed to execute step 4")
// 	}
// 	// Step 5 - unplug, flip & plug-in dongle then check all peripherals
// 	if err := dock89DonglePowerStep5(ctx, tconn, uc, dockingID); err != nil {
// 		return errors.Wrap(err,"failed to execute step 5")
// 	}
// 	// Step 6 - reboot then check peripherals
// 	if err := dock89DonglePowerStep6(ctx, true, uc); err != nil {
// 		return errors.Wrap(err,"failed to execute step 6")
// 	}
// 	// Step 7 - power off dongle & reboot, then check peripherals
// 	if err := dock89DonglePowerStep7(ctx, true, uc); err != nil {
// 		return errors.Wrap(err,"failed to execute step 7")
// 	}
// 	// Step 8 - check peripherals when power up / down couple times
// 	if err := dock89DonglePowerStep8(ctx, cr, uc); err != nil {
// 		return errors.Wrap(err,"failed to execute step 8")
// 	}
// 	// Step 9 - plug in dongle, sleep chromebook & wake up it, then check peripherals
// 	if err := dock89DonglePowerStep9(ctx, cr, uc, dockingID); err != nil {
// 		s.Fatal("Failed to execute step 9: ", err)
// 	}
// 	// Step 10 - plug in dongle, suspend chromebook, unplug dongle, wake up it, plug dongle, then check peripherls
// 	if err := dock89DonglePowerStep10(ctx, cr, uc, dockingID); err != nil {
// 		return errors.Wrap(err,"failed to execute step 1")
// 	}
// 	// 11. Unplug - Suspend - Plug - Resume // daq on - sleep - daq off - wake up - check peripherals
// 	if err := dock89DonglePowerStep11(ctx, cr, uc, dockingID); err != nil {
// 		s.Fatal("Failed to execute step 11: ", err)
// 	}
// 	return nil
// }

// usbDevices returns a list of USB devices. Each device is represented as a
// list of string. Each string contains some attributes related to the device.
func usbDevices(ctx context.Context) ([][]string, error) {
	const usbDevicesPath = "/sys/kernel/debug/usb/devices"
	b, err := ioutil.ReadFile(usbDevicesPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read file: %v", usbDevicesPath)
	}
	// /sys/kernel/debug/usb/devices looks like:
	//   [An empty line]
	//   T: Bus=01 Lev=00 Prnt=00 Port=00 Cnt=00 Dev#=  1 Spd=480 MxCh=16
	//   D: Ver= 2.00 Cls=09(hub  ) Sub=00 Prot=01 MxPS=64 #Cfgs=  1
	//   ...
	//   [Another empty line]
	//   T: ...
	//   D: ...
	//   ...
	// where an empty line represents start of device.
	var res [][]string
	sc := bufio.NewScanner(strings.NewReader(string(b)))
	for sc.Scan() {
		if sc.Text() == "" {
			res = append(res, []string{})
		} else {
			i := len(res) - 1
			res[i] = append(res[i], sc.Text())
		}
	}
	return res, nil
}

// For mocking.
var runCommand = func(ctx context.Context, cmd string, args ...string) ([]byte, error) {
	return testexec.CommandContext(ctx, cmd, args...).Output(testexec.DumpLogOnError)
}

// deviceNames returns the vendor name and the product name of device with
// vendorID:prodID. The names are extracted from lsusb.
func deviceNames(ctx context.Context, vendorID, prodID string) (string, string, error) {
	arg := fmt.Sprintf("-d%s:%s", vendorID, prodID)
	b, err := runCommand(ctx, "lsusb", "-v", arg)
	if err != nil {
		return "", "", err
	}
	lsusbOut := string(b)
	// Example output:
	//   Device Descriptor:
	//     ...
	//     idVendor           0x1d6b Linux Foundation
	//     idProduct          0x0003 3.0 root hub
	//     iManufacturer          2 Linux Foundation
	//     iProduct               3
	//     ...
	// We use these fields to get the names.
	reM := map[string]*regexp.Regexp{
		"iManufacturer": regexp.MustCompile(`^[ ]+iManufacturer[ ]+[\S]+([^\n]*)$`),
		"iProduct":      regexp.MustCompile(`^[ ]+iProduct[ ]+[\S]+([^\n]*)$`),
		"idVendor":      regexp.MustCompile(`^[ ]+idVendor[ ]+[\S]+([^\n]*)$`),
		"idProduct":     regexp.MustCompile(`^[ ]+idProduct[ ]+[\S]+([^\n]*)$`),
	}
	res := make(map[string]string)
	sc := bufio.NewScanner(strings.NewReader(lsusbOut))
	for sc.Scan() {
		for k, reg := range reM {
			m := reg.FindStringSubmatch(sc.Text())
			if m == nil {
				continue
			}
			if s := strings.Trim(m[1], " "); len(s) > 0 {
				res[k] = s
			}
		}
	}
	vendor, ok := res["idVendor"]
	if !ok {
		vendor, ok = res["iManufacturer"]
		if !ok {
			vendor = ""
		}
	}
	product, ok := res["idProduct"]
	if !ok {
		product, ok = res["iProduct"]
		if !ok {
			product = ""
		}
	}
	return vendor, product, nil
}
