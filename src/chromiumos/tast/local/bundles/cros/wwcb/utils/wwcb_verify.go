// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package utils provides funcs to cleanup folders in ChromeOS.
package utils

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/testing"
)

// ConnectState for connected status
type ConnectState bool

// fixture status
const (
	IsConnect    ConnectState = true
	IsDisconnect ConnectState = false
)

// VerifyExternalAudio verfiy external audio is connected or disconnected
func VerifyExternalAudio(ctx context.Context, s *testing.State, state ConnectState) error {

	s.Log("Start verifying external audio")

	// declare cras
	cras, err := audio.NewCras(ctx)
	if err != nil {

		return errors.Wrap(err, "failed to connect to cras")
	}

	// get nodes from cras
	nodes, err := cras.GetNodes(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to obtain cras nodes")
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

// VerifyEthernetStatus verify ethernet is connected or disconnected https://www.cyberciti.biz/faq/how-to-check-network-adapter-status-in-linux/
func VerifyEthernetStatus(ctx context.Context, s *testing.State, state ConnectState) error {

	s.Log("Start verifying ethernet status")

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
			return errors.Errorf("failed to check ethernet, got %s, want UP", output)
		}
	} else { // when ethernet is disconnect, cant get command shall output error
		if err == nil {
			return errors.New("When ethernet is disconnect, command shall be error")
		}
	}

	return nil
}

// VerifyPowerStatus verfiy power is charging or discharging
func VerifyPowerStatus(ctx context.Context, s *testing.State, state ConnectState) error {

	s.Log("Start verifying power status")

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

// VerifyExternalDisplay verify external display is connected or disconnected
func VerifyExternalDisplay(ctx context.Context, s *testing.State, tconn *chrome.TestConn, state ConnectState) error {

	s.Log("Start verifying external display")

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

		s.Log("Chromebook is in tablet mode, so there is no any external display")

		if len(infos) > 1 {
			return errors.New("Should unable to get any external display when chromebook is in tablet mode")
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
			return errors.Errorf("failed to verify external display status, got %t, want %t", currentStatus, wantStatus)
		}
	}

	return nil

}

// VerifyPeripherals verify all peripherals on station
func VerifyPeripherals(ctx context.Context, s *testing.State, tconn *chrome.TestConn, uc *UsbController, state ConnectState) error {

	s.Log("Start verifying all peripherals on station")

	// verify power
	if err := VerifyPowerStatus(ctx, s, state); err != nil {
		return err
	}

	testing.Sleep(ctx, time.Second)

	// verify external audio
	if err := VerifyExternalAudio(ctx, s, state); err != nil {
		return err
	}

	testing.Sleep(ctx, time.Second)

	// verify ethernet
	if err := VerifyEthernetStatus(ctx, s, state); err != nil {
		return err
	}

	testing.Sleep(ctx, time.Second)

	// verify ext-display
	if err := VerifyExternalDisplay(ctx, s, tconn, state); err != nil {
		return err
	}

	testing.Sleep(ctx, time.Second)

	// verify usb count
	if err := uc.VerifyUsbCount(ctx, s, state); err != nil {
		return err
	}

	return nil
}

// VerifyDisplayProperly verify display properly
// I think this method should not describe like this
// but co-worker say so
func VerifyDisplayProperly(ctx context.Context, s *testing.State, tconn *chrome.TestConn, want int) error {

	// get display info
	infos, err := display.GetInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get display info")
	}

	// 5. Check the external monitor display properly by test fixture.
	// 6. Check the chromebook display properly by test fixture.
	if len(infos) != want {
		return errors.Errorf("failed to check num of display, got %d, want %d", len(infos), want)
	}

	return nil
}

// VerifyDisplayCount verify display VerifyDisplayCount if is as expected
func VerifyDisplayCount(ctx context.Context, s *testing.State, tconn *chrome.TestConn, want int) error {

	// verification
	if err := testing.Poll(ctx, func(c context.Context) error {

		// get display
		infos, err := display.GetInfo(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get display info")
		}

		// check num of display
		if len(infos) < 2 {
			return errors.Errorf("failed to get enough display, got %d, at lest %d", len(infos), want)
		}

		// -  Now State:
		// *  Internal display will show up as (Primary)
		// *  External display will show up as (Extended)
		for _, info := range infos {
			// internal
			if info.IsInternal == true {
				if info.IsPrimary == false {
					return errors.Wrap(err, "Internal display should show up as (Primary)")
				}
			}
			// external
			if info.IsInternal == false {
				if info.IsPrimary == true {
					return errors.Wrap(err, "External display should show up as (Extended)")
				}
			}
		}

		return nil

	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to verify display status")
	}

	return nil
}

// VerifyKeyboard check keyboard works functionally
func VerifyKeyboard(ctx context.Context, s *testing.State) error {

	return nil
}

// VerifyMouse check mouse works functionally
func VerifyMouse(ctx context.Context, s *testing.State) error {

	return nil
}
