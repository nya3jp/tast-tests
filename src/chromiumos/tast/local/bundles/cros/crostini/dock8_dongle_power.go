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
package crostini

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/crostini/utils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Dock8DonglePower,
		Desc:         "USB Type-C multi-port dongle should work properly.",
		Contacts:     []string{"allion-sw@allion.com"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      10 * time.Minute,
		Vars:         utils.GetInputVars(),
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func Dock8DonglePower(ctx context.Context, s *testing.State) {

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

	// build usb recorder
	uc, err := utils.NewUsbController(ctx, s)
	if err != nil {
		s.Fatal("Failed to create usb recorder: ", err)
	}

	// step 1 - connect dongle when dongle is non-powered
	if err := Dock8DonglePower_Step1(ctx, s); err != nil {
		s.Fatal("Failed to execute step 1: ", err)
	}

	// step 2 - power up dongle
	if err := Dock8DonglePower_Step2(ctx, s); err != nil {
		s.Fatal("Failed to execute step 2: ", err)
	}

	// notice : if you check PerpDisplay1 here, may failed all the time
	// I don't know the reason for now
	// Step 3 - plug peripherals one by one, then check
	if err := Dock8DonglePower_Step3(ctx, s, tconn, uc); err != nil {
		s.Fatal("Failed to execute step 3: ", err)
	}

	// Step 4 - unplug & plug-in dongle then check all peripherals
	if err := Dock8DonglePower_Step4(ctx, s, tconn, uc); err != nil {
		s.Fatal("Failed to execute step 4: ", err)
	}

	// Step 5 - unplug, flip & plug-in dongle then check all peripherals
	if err := Dock8DonglePower_Step5(ctx, s, tconn, uc); err != nil {
		s.Fatal("Failed to execute step 5: ", err)
	}

	// Step 6 - reboot then check peripherals
	if err := Dock8DonglePower_Step6(ctx, s, false, uc); err != nil {
		s.Fatal("Failed to execute step 6: ", err)
	}

	// Step 7 - power off dongle & reboot, then check peripherals
	if err := Dock8DonglePower_Step7(ctx, s, false, uc); err != nil {
		s.Fatal("Failed to execute step 7: ", err)
	}

	// Step 8 - check peripherals when power up / down couple times
	if err := Dock8DonglePower_Step8(ctx, s, cr, uc); err != nil {
		s.Fatal("Failed to execute step 8: ", err)
	}

	// Step 9 - plug in dongle, sleep chromebook & wake up it, then check peripherals
	if err := Dock8DonglePower_Step9(ctx, s, cr, uc); err != nil {
		s.Fatal("Failed to execute step 9: ", err)
	}

	// Step 10 - plug in dongle, suspend chromebook, unplug dongle, wake up it, plug dongle, then check peripherls
	if err := Dock8DonglePower_Step10(ctx, s, cr, uc); err != nil {
		s.Fatal("Failed to execute step 10: ", err)
	}

	// Step 11 - unplug dongle, suspend chromebook, plug in dongle, wake up it, then check peripherals
	if err := Dock8DonglePower_Step11(ctx, s, cr, uc); err != nil {
		s.Fatal("Failed to execute step 11: ", err)
	}

	// Step 12 - into tablet, then repeat above steps
	if err := Dock8DonglePower_Step12(ctx, s, cr, uc); err != nil {
		s.Fatal("Failed to execute step 12: ", err)
	}
}

// 1. Hotplug the [non-powered] dongle alone // another fixture for type-c (TPE prepare) after daq off
func Dock8DonglePower_Step1(ctx context.Context, s *testing.State) error {

	s.Logf("Step 1 - Connect dongle when dongle is non-powered")

	// power off dongle
	if err := utils.SetStationPower(ctx, s, utils.StationPowerOff); err != nil {
		return errors.Wrapf(err, "Failed to power off dongle: ")
	}

	// plug in dongle
	if err := utils.ControlFixture(ctx, s, utils.FixtureStation, utils.ActionPlugin, false); err != nil {
		return errors.Wrap(err, "Failed to plug in dongle: ")
	}

	return nil
}

// 2. Power up the dongle (if support) // daq on
func Dock8DonglePower_Step2(ctx context.Context, s *testing.State) error {

	s.Logf("Step 2 - Power up dongle")

	// power up dongle
	if err := utils.SetStationPower(ctx, s, utils.StationPowerOn); err != nil {
		return errors.Wrapf(err, "Failed to power on dongle: ")
	}

	return nil
}

// 3. Hotplug peripheral(s) - one by one, or in combination - while dongle is plugged to Chromebook device // (TPE prepare fixture)
func Dock8DonglePower_Step3(ctx context.Context, s *testing.State, tconn *chrome.TestConn, uc *utils.UsbController) error {

	s.Logf("Step 3 - Plug peripherals one by one, then check")

	// plug peripherals one by one
	if err := utils.ControlPeripherals(ctx, s, uc, utils.ActionPlugin, false); err != nil {
		return errors.Wrap(err, "Failed to plug peripherals one by one: ")
	}

	// check peripherals
	if err := utils.VerifyPeripherals(ctx, s, tconn, uc, utils.IsConnect); err != nil {
		return errors.Wrap(err, "Failed to verify peripherals on dongle: ")
	}

	return nil
}

// 4. Unplug and plug-in back the dongle while all peripherals are plugged to the dongle // daq off then on, check peripherals using case 11 verification
func Dock8DonglePower_Step4(ctx context.Context, s *testing.State, tconn *chrome.TestConn, uc *utils.UsbController) error {

	s.Logf("Step 4 - Unplug & plug-in dongle then check all peripherals")

	// unplug dongle
	if err := utils.ControlFixture(ctx, s, utils.FixtureStation, utils.ActionUnplug, false); err != nil {
		return errors.Wrap(err, "Failed to unplug dongle: ")
	}

	// plug-in dongle
	if err := utils.ControlFixture(ctx, s, utils.FixtureStation, utils.ActionPlugin, false); err != nil {
		return errors.Wrap(err, "Failed to plug in dongle: ")
	}

	// check peripherals
	if err := utils.VerifyPeripherals(ctx, s, tconn, uc, utils.IsConnect); err != nil {
		return errors.Wrap(err, "Failed to check peripherals on dongle: ")
	}

	return nil
}

// 5. Unplug, Flip, and plug-in back the dongle while all peripherals are plugged to the dongle // flip( James perpare) then check
func Dock8DonglePower_Step5(ctx context.Context, s *testing.State, tconn *chrome.TestConn, uc *utils.UsbController) error {

	s.Logf("Step 5 - Unplug, flip & plug-in dongle then check all peripherals")

	// unplug dongle
	if err := utils.ControlFixture(ctx, s, utils.FixtureStation, utils.ActionUnplug, false); err != nil {
		return errors.Wrap(err, "Failed to unplug dongle: ")
	}

	// flip dongle
	if err := utils.ControlFixture(ctx, s, utils.FixtureStation, utils.ActionFlip, false); err != nil {
		return errors.Wrap(err, "Failed to flip dongle: ")
	}

	// plug in dongle
	if err := utils.ControlFixture(ctx, s, utils.FixtureStation, utils.ActionPlugin, false); err != nil {
		return errors.Wrap(err, "Failed to plug in dongle: ")
	}

	// check peripherals
	if err := utils.VerifyPeripherals(ctx, s, tconn, uc, utils.IsConnect); err != nil {
		return errors.Wrap(err, "Failed to check peripherals on dongle: ")
	}

	return nil
}

// 6. Reboot the Chromebook device with peripherals connected to the powered dongle. // reboot then check
func Dock8DonglePower_Step6(ctx context.Context, s *testing.State, intoTablet bool, uc *utils.UsbController) error {

	s.Logf("Step 6 - Reboot then check peripherals")

	cr, err := chrome.New(ctx, chrome.ARCEnabled())
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}

	defer func() {
		if err := cr.Close(ctx); err != nil {
			s.Fatal("Failed to close Chrome while (re)booting ARC: ", err)
		}
	}()

	// re-create api connection
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "Failed to create Test API connection: ")
	}

	// into tablet mode,  mode change back cuz reboot
	if intoTablet == true {
		_, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
		if err != nil {
			s.Fatal("Failed to ensure in tablet mode: ", err)
		}
		// defer cleanup(ctx)
	}

	// check peripherals
	if err := utils.VerifyPeripherals(ctx, s, tconn, uc, utils.IsConnect); err != nil {
		return errors.Wrap(err, "Failed to check peripherals on dongle: ")
	}

	return nil
}

// 7. Remove power from dongle and reboot Chromebook device // daq off then reboot then check
func Dock8DonglePower_Step7(ctx context.Context, s *testing.State, intoTablet bool, uc *utils.UsbController) error {

	s.Logf("Step 7 - Power off dongle & reboot, then check peripherals")

	// power off dongle
	if err := utils.SetStationPower(ctx, s, utils.StationPowerOff); err != nil {
		return errors.Wrapf(err, "Failed to power off dongle: ")
	}

	// reboot
	cr, err := chrome.New(ctx, chrome.ARCEnabled())
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}

	defer func() {
		if err := cr.Close(ctx); err != nil {
			s.Fatal("Failed to close Chrome while (re)booting ARC: ", err)
		}
	}()

	// re-create api connection
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "Failed to create Test API connection: ")
	}

	if intoTablet == true {
		_, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
		if err != nil {
			s.Fatal("Failed to ensure in tablet mode: ", err)
		}
		// defer cleanup(ctx)
	}

	// check peripherals is not on dongle
	if err := utils.VerifyPeripherals(ctx, s, tconn, uc, utils.IsDisconnect); err != nil {
		return errors.Wrap(err, "Failed to check peripherals is not on dongle: ")
	}

	return nil
}

// 8. Power up, Power down, and Power up again the dongle while all ports are busy with Ext-Display and HID (Keyboard, or Mouse) // daq on then check, daq off then on then check (moniter, keyboard, mouse)
func Dock8DonglePower_Step8(ctx context.Context, s *testing.State, cr *chrome.Chrome, uc *utils.UsbController) error {

	s.Logf("Step 8 - Check peripherals when power up / down couple times  ")

	if err := cr.Reconnect(ctx); err != nil {
		return errors.Wrap(err, "Failed to reconnect to chromebook: ")
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "Failed to create Test API connection: ")
	}

	// power up
	if err := utils.SetStationPower(ctx, s, utils.StationPowerOn); err != nil {
		return errors.Wrapf(err, "Failed to power on dongle: ")
	}

	// check peripherals are on station
	if err := utils.VerifyPeripherals(ctx, s, tconn, uc, utils.IsConnect); err != nil {
		return errors.Wrap(err, "Failed to check peripherals on dongle: ")
	}

	// power down
	if err := utils.SetStationPower(ctx, s, utils.StationPowerOff); err != nil {
		return errors.Wrapf(err, "Failed to power off dongle: ")
	}

	// check peripherals are not on station
	if err := utils.VerifyPeripherals(ctx, s, tconn, uc, utils.IsDisconnect); err != nil {
		return errors.Wrap(err, "Failed to check peripherals on dongle: ")
	}

	// power up
	if err := utils.SetStationPower(ctx, s, utils.StationPowerOn); err != nil {
		return errors.Wrapf(err, "Failed to power on dongle: ")
	}

	// check peripherals are on station
	if err := utils.VerifyPeripherals(ctx, s, tconn, uc, utils.IsConnect); err != nil {
		return errors.Wrap(err, "Failed to check peripherals on dongle: ")
	}

	return nil
}

// 9. Plug - Suspend - Resume . // daq on then sleep then wake up then check
func Dock8DonglePower_Step9(ctx context.Context, s *testing.State, cr *chrome.Chrome, uc *utils.UsbController) error {

	s.Logf("Step 9 - Plug in dongle, sleep chromebook & wake up it, then check peripherals")

	// plug in
	if err := utils.ControlFixture(ctx, s, utils.FixtureStation, utils.ActionPlugin, false); err != nil {
		return errors.Wrapf(err, "Failed to plug in dongle: ")
	}

	// suspend then reconnect chromebook
	tconn, err := utils.SuspendChromebook(ctx, s, cr)
	if err != nil {
		return errors.Wrap(err, "Failed to suspend then reconnect chromebook: ")
	}

	// check peripherals
	if err := utils.VerifyPeripherals(ctx, s, tconn, uc, utils.IsConnect); err != nil {
		return errors.Wrap(err, "Failed to check peripherals on dongle: ")
	}

	return nil
}

// 10. Plug - Suspend - Unplug - Resume - Plug // daq on then sleep then daq off then wake up then daq on then check
func Dock8DonglePower_Step10(ctx context.Context, s *testing.State, cr *chrome.Chrome, uc *utils.UsbController) error {

	s.Logf("Step 10 - Plug in dongle, suspend chromebook, unplug dongle, wake up it, plug dongle, then check peripherls")

	// plug in dongle
	if err := utils.ControlFixture(ctx, s, utils.FixtureStation, utils.ActionPlugin, false); err != nil {
		return errors.Wrapf(err, "Failed to plug in dongle: ")
	}

	// unplug dongle later
	if err := utils.ControlFixture(ctx, s, utils.FixtureStation, utils.ActionUnplug, true); err != nil {
		return errors.Wrap(err, "Failed to unplug dongle later: ")
	}

	// suspend then wake up chromebook
	tconn, err := utils.SuspendChromebook(ctx, s, cr)
	if err != nil {
		return errors.Wrap(err, "Failed to suspend then reconnect chromebook: ")
	}

	// plug in dongle
	if err := utils.ControlFixture(ctx, s, utils.FixtureStation, utils.ActionPlugin, false); err != nil {
		return errors.Wrapf(err, "Failed to plug in dongle: ")
	}

	// check peripherals
	if err := utils.VerifyPeripherals(ctx, s, tconn, uc, utils.IsConnect); err != nil {
		return errors.Wrap(err, "Failed to check peripherals on dongle: ")
	}

	return nil
}

// 11. Unplug - Suspend - Plug - Resume // daq on - sleep - daq off - wake up - check peripherals
func Dock8DonglePower_Step11(ctx context.Context, s *testing.State, cr *chrome.Chrome, uc *utils.UsbController) error {
	s.Logf("Step 11 - Unplug dongle, suspend chromebook, plug in dongle, wake up it, then check peripherals")

	// unplug dongle
	if err := utils.ControlFixture(ctx, s, utils.FixtureStation, utils.ActionUnplug, false); err != nil {
		return errors.Wrapf(err, "Failed to unplug dongle: ")
	}

	// plug in dongle later
	if err := utils.ControlFixture(ctx, s, utils.FixtureStation, utils.ActionPlugin, true); err != nil {
		return errors.Wrap(err, "Failed to plug in dongle later: ")
	}

	// suspend then wake up chromebook
	tconn, err := utils.SuspendChromebook(ctx, s, cr)
	if err != nil {
		return errors.Wrap(err, "Failed to suspend then reconnect chromebook: ")
	}

	// check peripherals
	if err := utils.VerifyPeripherals(ctx, s, tconn, uc, utils.IsConnect); err != nil {
		return errors.Wrap(err, "Failed to check peripherals on dongle: ")
	}

	return nil
}

// into tablet mode, repeat above steps
func Dock8DonglePower_Step12(ctx context.Context, s *testing.State, cr *chrome.Chrome, uc *utils.UsbController) error {

	s.Logf("Step 12 - into tablet mode, repeat above steps ")

	if err := cr.Reconnect(ctx); err != nil {
		return errors.Wrap(err, "Failed to reconnect to chromebook: ")
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "Failed to create Test API connection: ")
	}

	// into tablet mode, repeat above steps
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
	if err != nil {
		s.Fatal("Failed to ensure in tablet mode: ", err)
	}
	defer cleanup(ctx)

	// repeat above steps

	// step 1 - connect dongle when dongle is non-powered
	if err := Dock8DonglePower_Step1(ctx, s); err != nil {
		s.Fatal("Failed to execute step 1 ", err)
	}

	// step 2 - power up dongle
	if err := Dock8DonglePower_Step2(ctx, s); err != nil {
		s.Fatal("Failed to execute step 2 ", err)
	}

	// Step 3 - plug peripherals one by one, then check
	if err := Dock8DonglePower_Step3(ctx, s, tconn, uc); err != nil {
		s.Fatal("Failed to execute step 3 ", err)
	}

	// Step 4 - unplug & plug-in dongle then check all peripherals
	if err := Dock8DonglePower_Step4(ctx, s, tconn, uc); err != nil {
		s.Fatal("Failed to execute step 4 ", err)
	}

	// Step 5 - unplug, flip & plug-in dongle then check all peripherals
	if err := Dock8DonglePower_Step5(ctx, s, tconn, uc); err != nil {
		s.Fatal("Failed to execute step 5 ", err)
	}

	// Step 6 - reboot then check peripherals
	if err := Dock8DonglePower_Step6(ctx, s, true, uc); err != nil {
		s.Fatal("Failed to execute step 6 ", err)
	}

	// Step 7 - power off dongle & reboot, then check peripherals
	if err := Dock8DonglePower_Step7(ctx, s, true, uc); err != nil {
		s.Fatal("Failed to execute step 7 ", err)
	}

	// Step 8 - check peripherals when power up / down couple times
	if err := Dock8DonglePower_Step8(ctx, s, cr, uc); err != nil {
		s.Fatal("Failed to execute step 8 ", err)
	}

	// Step 9 - plug in dongle, sleep chromebook & wake up it, then check peripherals
	if err := Dock8DonglePower_Step9(ctx, s, cr, uc); err != nil {
		s.Fatal("Failed to execute step 9 ", err)
	}

	// Step 10 - plug in dongle, suspend chromebook, unplug dongle, wake up it, plug dongle, then check peripherls
	if err := Dock8DonglePower_Step10(ctx, s, cr, uc); err != nil {
		s.Fatal("Failed to execute step 10 ", err)
	}

	// 11. Unplug - Suspend - Plug - Resume // daq on - sleep - daq off - wake up - check peripherals
	if err := Dock8DonglePower_Step11(ctx, s, cr, uc); err != nil {
		s.Fatal("Failed to execute step 11 ", err)
	}

	return nil
}
