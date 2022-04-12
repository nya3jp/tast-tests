// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package utils

import (
	"context"
	"io/ioutil"
	"strings"
	"time"

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
// by finding out audio devices have "USB" type
func VerifyExternalAudio(ctx context.Context, wantState ConnectState) error {
	testing.ContextLog(ctx, "Start verifying external audio")
	cras, err := audio.NewCras(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create cras")
	}
	// find exteranl audio device by checking there is "USB" type
	return testing.Poll(ctx, func(c context.Context) error {
		var currentStatus bool
		currentStatus = false
		nodes, err := cras.GetNodes(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get nodes from cras")
		}
		for _, n := range nodes {
			if n.Type == "USB" {
				currentStatus = true
				break
			}
		}
		wantStatus := bool(wantState)
		if currentStatus != wantStatus {
			return errors.Errorf("Searching ext-audio result is not match; got %t, want %t", currentStatus, wantStatus)
		}
		return nil
	}, &testing.PollOptions{Timeout: AudioTimeout, Interval: time.Second})
}

// VerifyEthernetStatus verify ethernet is connected or disconnected https://www.cyberciti.biz/faq/how-to-check-network-adapter-status-in-linux/
// default: set "eth0" as ethernet connect input
// "eth0" only show up when docking station is connect
// "wlan0" always show up no matter docking station is connect or disconnect
func VerifyEthernetStatus(ctx context.Context, wantState ConnectState) error {
	testing.ContextLog(ctx, "Start verifying ethernet status")
	// check ethernet state is matched
	return testing.Poll(ctx, func(c context.Context) error {
		output, err := ioutil.ReadFile("/sys/class/net/eth0/operstate")
		if err != nil {
			if wantState {
				return errors.Wrap(err, "failed to get eth0 operstate")
			}
			// When chromebook eth0 is not exist, define as ethernet is disconnected
			return nil
		}
		if wantState {
			if "UP" != string(output) {
				return errors.Errorf("Ethernet status is not match; want up, got %s", string(output))
			}
		} else {
			if "DOWN" != string(output) {
				return errors.Errorf("Ethernet status is not match; want down, got %s", string(output))
			}
		}
		return nil
	}, &testing.PollOptions{Timeout: EthernetTimeout, Interval: time.Second})
}

// VerifyPowerStatus verfiy power is charging or discharging
func VerifyPowerStatus(ctx context.Context, wantState ConnectState) error {
	testing.ContextLog(ctx, "Start verifying power status")
	var wantStatus string
	if wantState {
		wantStatus = "CHARGING"
	} else {
		wantStatus = "DISCHARGING"
	}
	// check power status is matched
	return testing.Poll(ctx, func(c context.Context) error {
		output, err := ioutil.ReadFile("/sys/class/power_supply/BAT0/status")
		if err != nil {
			return errors.Wrap(err, "failed to get power state")
		}
		currentStatus := strings.ToUpper(strings.TrimSpace(string(output)))
		if wantStatus != currentStatus {
			return errors.Errorf("Power status is not match; got %s, want %s", currentStatus, wantStatus)
		}
		return nil
	}, &testing.PollOptions{Timeout: PowerTimeout, Interval: time.Second})
}

// VerifyDisplayProperly verify display properly
// use this func when face "Check the chromebook or external display properly by test fixture." due to testing requirements
func VerifyDisplayProperly(ctx context.Context, tconn *chrome.TestConn, want int) error {
	return testing.Poll(ctx, func(c context.Context) error {
		infos, err := display.GetInfo(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get display info")
		}
		if len(infos) != want {
			return errors.Errorf("failed to get correct number of display; got %d, want %d", len(infos), want)
		}
		return nil
	}, &testing.PollOptions{Timeout: DisplayTimeout, Interval: time.Second})
}

// VerifyDisplayState verify display state;
// Internal display will show up as (Primary)
// External display will show up as (Extended)
func VerifyDisplayState(ctx context.Context, tconn *chrome.TestConn) error {
	return testing.Poll(ctx, func(c context.Context) error {
		infos, err := GetInternalAndExternalDisplays(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get internal & external display")
		}
		if !infos.Internal.IsPrimary {
			return errors.Wrap(err, "Internal display should show up as primary")
		}
		if infos.External.IsPrimary {
			return errors.Wrap(err, "External display should show up as extended")
		}
		return nil
	}, &testing.PollOptions{Timeout: DisplayTimeout, Interval: time.Second})
}

// VerifyPeripherals verify all peripherals on station
func VerifyPeripherals(ctx context.Context, tconn *chrome.TestConn, uc *UsbController, wantState ConnectState) error {
	testing.ContextLog(ctx, "Start verifying all peripherals on station")
	// verify power
	if err := VerifyPowerStatus(ctx, wantState); err != nil {
		return err
	}
	// verify external audio
	if err := VerifyExternalAudio(ctx, wantState); err != nil {
		return err
	}
	// verify ethernet
	if err := VerifyEthernetStatus(ctx, wantState); err != nil {
		return err
	}
	// verify ext-display
	if err := VerifyExternalDisplay(ctx, tconn, wantState); err != nil {
		return err
	}
	// verify usb count
	if err := uc.VerifyUsbCount(ctx, wantState); err != nil {
		return err
	}
	return nil
}

// VerifyExternalDisplay verify external display is connected or disconnected
func VerifyExternalDisplay(ctx context.Context, tconn *chrome.TestConn, wantState ConnectState) error {
	testing.ContextLog(ctx, "Start verifying external display")

	isTabletModeEnabled, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		return err
	}

	infos, err := display.GetInfo(ctx, tconn)
	if err != nil {
		return err
	}

	// check currect status is tablet mode
	if isTabletModeEnabled {
		testing.ContextLog(ctx, "Chromebook is in tablet mode, so there is no any external display")
		if len(infos) > 1 {
			return errors.New("Should unable to get any external display when chromebook is in tablet mode")
		}
	} else {
		currentStatus := false
		for _, info := range infos {
			if !info.IsInternal {
				currentStatus = true
				break
			}
		}
		wantStatus := bool(wantState)
		if currentStatus != wantStatus {
			return errors.Errorf("failed to verify external display status; got %t, want %t", currentStatus, wantStatus)
		}
	}
	return nil
}
