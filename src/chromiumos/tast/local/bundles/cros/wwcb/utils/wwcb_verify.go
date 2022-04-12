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

	// find audio device has "USB" type
	var currentStatus bool
	currentStatus = false
	cras, err := audio.NewCras(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create cras")
	}
	nodes, err := cras.GetNodes(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get nodes from cras")
	}
	for _, n := range nodes {
		if n.Type == "USB" {
			currentStatus = true
		}
	}

	wantStatus := bool(wantState)
	// check status
	if currentStatus != wantStatus {
		return errors.Errorf("Searching ext-audio result is not match; got %t, want %t", currentStatus, wantStatus)
	}

	return nil
}

// VerifyEthernetStatus verify ethernet is connected or disconnected https://www.cyberciti.biz/faq/how-to-check-network-adapter-status-in-linux/
// default: set "eth0" as ethernet connect input
// "eth0" only show up when docking station is connect
// "wlan0" always show up no matter docking station is connect or disconnect
func VerifyEthernetStatus(ctx context.Context, wantState ConnectState) error {
	testing.ContextLog(ctx, "Start verifying ethernet status")
	// get current ethernet status
	output, err := ioutil.ReadFile("/sys/class/net/eth0/operstate")
	// verify error
	if wantState {
		if err != nil {
			return errors.Wrap(err, "failed to get eth0 operstate")
		}
	} else {
		// when eth0 is not exist, define as ethernet is disconnected
		if err != nil {
			return nil
		}
	}
	// when ethernet is connected, check ethernet status is "UP", not "DOWN"
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
	// get current power status
	output, err := ioutil.ReadFile("/sys/class/power_supply/BAT0/status")
	if err != nil {
		return errors.Wrap(err, "failed to get power state")
	}
	// when power is connected, check power status is "CHARGING", not "DISCHARGING"
	currentStatus := strings.ToUpper(strings.TrimSpace(string(output)))
	if wantStatus != currentStatus {
		return errors.Errorf("Power status is not match; got %s, want %s", currentStatus, wantStatus)
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
		var currentStatus bool
		currentStatus = false
		for _, info := range infos {
			if !info.IsInternal {
				currentStatus = true
			}
		}
		wantStatus := bool(wantState)
		if currentStatus != wantStatus {
			return errors.Errorf("failed to verify external display status; got %t, want %t", currentStatus, wantStatus)
		}
	}
	return nil
}

// VerifyDisplayProperly verify display properly
// use this func when face "Check the chromebook or external display properly by test fixture." due to testing requirements
func VerifyDisplayProperly(ctx context.Context, tconn *chrome.TestConn, want int) error {
	if err := testing.Poll(ctx, func(c context.Context) error {
		infos, err := display.GetInfo(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get display info")
		}
		if len(infos) != want {
			return errors.Errorf("failed to get correct number of display; got %d, want %d", len(infos), want)
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to verify display properly")
	}
	return nil
}

// VerifyDisplayState verify display state;
// Internal display will show up as (Primary)
// External display will show up as (Extended)
func VerifyDisplayState(ctx context.Context, tconn *chrome.TestConn) error {
	if err := testing.Poll(ctx, func(c context.Context) error {
		infos, err := display.GetInfo(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get display info")
		}
		for _, info := range infos {
			if info.IsInternal { // internal
				if !info.IsPrimary {
					return errors.Wrap(err, "Internal display should show up as primary")
				}
			}
			if !info.IsInternal { // external
				if info.IsPrimary {
					return errors.Wrap(err, "External display should show up as extended")
				}
			}
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to verify display state")
	}
	return nil
}
