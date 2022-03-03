package utils

import (
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/testing"
	"context"
	"strings"
	"time"
)

type ConnectState bool

const (
	IsConnect    ConnectState = true
	IsDisconnect ConnectState = false
)

// ex: Realtek CE USB Audio: USB Audio:2,0
// verfiy external audio is connected or disconnected
func VerifyExternalAudio(ctx context.Context, s *testing.State, state ConnectState) error {

	s.Logf("Start verifying external audio")

	// declare cras
	cras, err := audio.NewCras(ctx)
	if err != nil {

		return errors.Errorf("Failed to connect to cras: ", err)
	}

	// get nodes from cras
	nodes, err := cras.GetNodes(ctx)
	if err != nil {
		return errors.Errorf("Failed to obtain cras nodes: ", err)
	}

	// find ext-audio device is connect or not
	var currentStatus bool
	currentStatus = false
	for _, n := range nodes {
		if n.Type == "USB" {
			currentStatus = true
		}
	}

	wantStatus := bool(state)
	// check status
	if currentStatus != wantStatus {
		return errors.Errorf("Searching ext-audio result is not match, got %t, want %t", currentStatus, wantStatus)
	}

	return nil
}

// https://www.cyberciti.biz/faq/how-to-check-network-adapter-status-in-linux/
// sudo lshw -class network -short

// ex: eth0
// check ethernet on docking station

// verify ethernet is connected or disconnected
func VerifyEthernetStatus(ctx context.Context, s *testing.State, state ConnectState) error {

	s.Logf("Start verifying ethernet status")

	command := testexec.CommandContext(ctx, "cat", "/sys/class/net/eth0/operstate")

	s.Logf("%s", command)

	output, err := command.Output(testexec.DumpLogOnError)

	// when ethernet is connected, check ethernet status is "UP", not "DOWN"
	if bool(state) {
		// check error
		if err != nil {
			return err
		}

		// check status
		if strings.ToUpper(strings.TrimSpace(string(output))) != "UP" {
			return errors.Errorf("Failed to check ethernet, got %s, want %s,")
		}
	} else { // when ethernet is disconnect, cant get command shall output error
		if err == nil {
			return errors.Errorf("When ethernet is disconnect, command shall be error")
		}
	}

	return nil
}

// verfiy power is charging or discharging
func VerifyPowerStatus(ctx context.Context, s *testing.State, state ConnectState) error {

	s.Logf("Start verifying power status")

	// define expect state to check
	var wantStatus string
	if state {
		wantStatus = "CHARGING"
	} else {
		wantStatus = "DISCHARGING"
	}

	command := testexec.CommandContext(ctx, "cat", "/sys/class/power_supply/BAT0/status")

	s.Logf("%s", command)

	output, err := command.Output(testexec.DumpLogOnError)
	if err != nil {
		return err
	}

	// check currentStatus is match condition
	currentStatus := strings.ToUpper(strings.TrimSpace(string(output)))
	if currentStatus != wantStatus {
		return errors.Errorf("Power status is not match, got %s, want %s", currentStatus, wantStatus)
	}

	return nil
}

// verify external display is connected or disconnected
func VerifyExternalDisplay(ctx context.Context, s *testing.State, tconn *chrome.TestConn, state ConnectState) error {

	s.Logf("Start verifying external display")

	isTabletModeEnabled, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		return err
	}

	infos, err := display.GetInfo(ctx, tconn)
	if err != nil {
		return err
	}

	// check currect status is tablet mode
	if isTabletModeEnabled == true {

		s.Logf("Chromebook is in tablet mode, so there is no any external display")

		if len(infos) > 1 {
			return errors.Errorf("Should unable to get any external display when chromebook is in tablet mode")
		}

	} else {

		var currentStatus bool
		currentStatus = false
		for _, info := range infos {
			if info.IsInternal == false {
				currentStatus = true
			}
		}

		wantStatus := bool(state)
		if currentStatus != wantStatus {
			return errors.Errorf("Failed to verify external display status, got %t, want %t", currentStatus, wantStatus)
		}
	}

	return nil

}

// verify all peripherals on station
func VerifyPeripherals(ctx context.Context, s *testing.State, tconn *chrome.TestConn, uc *UsbController, state ConnectState) error {

	s.Logf("Start verifying peripherals")

	// if err := testing.Poll(ctx, func(ctx context.Context) error {
	// verify power
	if err := VerifyPowerStatus(ctx, s, state); err != nil {
		return err
	}

	time.Sleep(1 * time.Second)

	// verify external audio
	if err := VerifyExternalAudio(ctx, s, state); err != nil {
		return err
	}

	time.Sleep(1 * time.Second)

	// verify ethernet
	if err := VerifyEthernetStatus(ctx, s, state); err != nil {
		return err
	}

	time.Sleep(1 * time.Second)

	// verify ext-display
	if err := VerifyExternalDisplay(ctx, s, tconn, state); err != nil {
		return err
	}

	time.Sleep(1 * time.Second)

	// verify usb count
	if err := uc.VerifyUsbCount(ctx, s, state); err != nil {
		return err
	}

	// 	return nil
	// }, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
	// 	return err
	// }

	return nil
}

// verify display properly
func VerifyDisplayProperly(ctx context.Context, s *testing.State, tconn *chrome.TestConn, want int) error {

	// get display info
	infos, err := display.GetInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "Failed to get display info")
	}

	// 5. Check the external monitor display properly by test fixture.
	// 6. Check the chromebook display properly by test fixture.
	if len(infos) != want {
		return errors.Errorf("Failed to check num of display, got %d, want %d", len(infos), want)
	}

	return nil
}
