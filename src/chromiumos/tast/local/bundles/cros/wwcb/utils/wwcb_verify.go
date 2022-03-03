// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package utils

import (
	"context"
	"io/ioutil"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/testing"
)

// VerifyExternalAudio verifies external audio is connected or disconnected.
func VerifyExternalAudio(ctx context.Context, isConnect bool) error {
	testing.ContextLog(ctx, "Start verifying external audio")
	cras, err := audio.NewCras(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create cras")
	}
	// Find out external audio device with USB type.
	return testing.Poll(ctx, func(c context.Context) error {
		nodes, err := cras.GetNodes(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get nodes from cras")
		}
		extAudioState := false
		for _, n := range nodes {
			if n.Type == "USB" {
				extAudioState = true
				break
			}
		}
		if extAudioState != isConnect {
			return errors.Errorf("unexpected ext-audio presenting state: got %t, want %t", extAudioState, isConnect)
		}
		return nil
	}, &testing.PollOptions{Timeout: AudioTimeout, Interval: AudioInterval})
}

// VerifyEthernetStatus verifies ethernet is connected or disconnected.
func VerifyEthernetStatus(ctx context.Context, isConnect bool) error {
	testing.ContextLog(ctx, "Start verifying ethernet status")
	var wantEthOperState string
	if isConnect {
		wantEthOperState = "UP"
	} else {
		wantEthOperState = "DOWN"
	}
	// Find out eth0 operstate.
	return testing.Poll(ctx, func(c context.Context) error {
		output, err := ioutil.ReadFile("/sys/class/net/eth0/operstate")
		if err != nil {
			if isConnect {
				return errors.Wrap(err, "failed to get eth0 operstate")
			}
			// When eth0 is not exist, consider ethernet as disconnected.
			return nil
		}
		currentState := strings.ToUpper(strings.TrimSpace(string(output)))
		if wantEthOperState != currentState {
			return errors.Errorf("unexpected ethernet status: want %s, got %s", wantEthOperState, currentState)
		}
		return nil
	}, &testing.PollOptions{Timeout: EthernetTimeout, Interval: EthernetInterval})
}

// VerifyPowerStatus verifies power is charging or discharging.
func VerifyPowerStatus(ctx context.Context, isConnect bool) error {
	testing.ContextLog(ctx, "Start verifying power status")
	var wantPowerStatus string
	if isConnect {
		wantPowerStatus = "CHARGING"
	} else {
		wantPowerStatus = "DISCHARGING"
	}
	// Find out BAT0 status.
	return testing.Poll(ctx, func(c context.Context) error {
		output, err := ioutil.ReadFile("/sys/class/power_supply/BAT0/status")
		if err != nil {
			return errors.Wrap(err, "failed to get power status")
		}
		currentStatus := strings.ToUpper(strings.TrimSpace(string(output)))
		if wantPowerStatus != currentStatus {
			return errors.Errorf("unexpected power status: got %s, want %s", currentStatus, wantPowerStatus)
		}
		return nil
	}, &testing.PollOptions{Timeout: PowerTimeout, Interval: PowerInterval})
}

// VerifyExternalDisplay verifies external display is connected or disconnected.
func VerifyExternalDisplay(ctx context.Context, tconn *chrome.TestConn, isConnect bool) error {
	testing.ContextLog(ctx, "Start verifying external display")
	return testing.Poll(ctx, func(c context.Context) error {
		// There is no external display info when Chromebook is into tablet mode.
		isTabletModeEnabled, err := ash.TabletModeEnabled(ctx, tconn)
		if err != nil {
			return err
		} else if isTabletModeEnabled {
			return nil
		}

		_, err = GetInternalAndExternalDisplays(ctx, tconn)
		if isConnect {
			if err != nil {
				return err
			}
		} else {
			if err == nil {
				return errors.New("unexpected external display detected")
			}
		}
		return nil
	}, &testing.PollOptions{Timeout: DisplayTimeout, Interval: DisplayInterval})
}

// VerifyPeripherals verifies all peripherals is connected or disconnected.
func VerifyPeripherals(ctx context.Context, tconn *chrome.TestConn, uc *UsbController, isConnect bool) error {
	testing.ContextLog(ctx, "Start verifying all peripherals")

	if err := VerifyPowerStatus(ctx, isConnect); err != nil {
		return err
	}

	if err := VerifyEthernetStatus(ctx, isConnect); err != nil {
		return err
	}

	if err := VerifyExternalDisplay(ctx, tconn, isConnect); err != nil {
		return err
	}

	if err := VerifyExternalAudio(ctx, isConnect); err != nil {
		return err
	}

	if err := uc.VerifyUsbCount(ctx, isConnect); err != nil {
		return err
	}
	return nil
}

// VerifyDisplayCount verifies number of displays.
func VerifyDisplayCount(ctx context.Context, tconn *chrome.TestConn, count int) error {
	return testing.Poll(ctx, func(c context.Context) error {
		infos, err := display.GetInfo(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get display info")
		}
		if len(infos) != count {
			return errors.Errorf("unexpected number of displays: got %d, want %d", len(infos), count)
		}
		return nil
	}, &testing.PollOptions{Timeout: DisplayTimeout, Interval: DisplayInterval})
}

// VerifyDisplayState verifies display state.
// Internal display show up as primary.
// External display show up as extended.
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
	}, &testing.PollOptions{Timeout: DisplayTimeout, Interval: DisplayInterval})
}
