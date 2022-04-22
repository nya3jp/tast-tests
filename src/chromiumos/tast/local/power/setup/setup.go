// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package setup contains helpers to set up a DUT for a power test.
package setup

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// CleanupCallback cleans up a single setup item.
type CleanupCallback func(context.Context) error

// Nested is used by setup items that have multiple stages that need separate
// cleanup callbacks.
func Nested(ctx context.Context, name string, nestedSetup func(s *Setup) error) (CleanupCallback, error) {
	s, callback := New(name)
	succeeded := false
	defer func() {
		if !succeeded {
			callback(ctx)
		}
	}()
	testing.ContextLogf(ctx, "Setting up %q", name)
	if err := nestedSetup(s); err != nil {
		return nil, err
	}
	if err := s.Check(ctx); err != nil {
		return nil, err
	}
	succeeded = true
	return callback, nil
}

// Setup accumulates the results of setup items so that their results can be
// checked, errors logged, and cleaned up.
type Setup struct {
	name      string
	callbacks []CleanupCallback
	errs      []error
}

// New creates a Setup object to collect the results of setup items, and a
// cleanup function that should be immediately deferred to make sure cleanup
// callbacks are called.
func New(name string) (*Setup, CleanupCallback) {
	s := &Setup{
		name:      name,
		callbacks: nil,
		errs:      nil,
	}
	cleanedUp := false
	return s, func(ctx context.Context) error {
		if cleanedUp {
			return errors.Errorf("cleanup %q has already been called", name)
		}
		cleanedUp = true

		if count, err := s.cleanUp(ctx); err != nil {
			return errors.Wrapf(err, "cleanup %q had %d items fail, first failure", name, count)
		}
		return nil
	}
}

// cleanUp is a helper that runs all cleanup callbacks and logs any failures.
// Returns true if all cleanup
func (s *Setup) cleanUp(ctx context.Context) (errorCount int, firstError error) {
	for _, c := range s.callbacks {
		// Defer cleanup calls so that if any of them panic, the rest still run.
		defer func(callback CleanupCallback) {
			if err := callback(ctx); err != nil {
				errorCount++
				if firstError == nil {
					firstError = err
				}
				testing.ContextLogf(ctx, "Cleanup %q failed: %s", s.name, err)
			}
		}(c)
	}
	return 0, nil
}

// Add adds a result to be checked or cleaned up later.
func (s *Setup) Add(callback CleanupCallback, err error) {
	if callback != nil {
		s.callbacks = append(s.callbacks, callback)
	}
	if err != nil {
		s.errs = append(s.errs, err)
	}
}

// Check checks if any Result shows a failure happened. All failures are logged,
// and a summary of failures is returned.
func (s *Setup) Check(ctx context.Context) error {
	for _, err := range s.errs {
		testing.ContextLogf(ctx, "Setup %q failed: %s", s.name, err)
	}
	if len(s.errs) > 0 {
		return errors.Wrapf(s.errs[0], "setup %q had %d items fail, first failure", s.name, len(s.errs))
	}
	return nil
}

// PowerdMode indicates what powerd setup is needed for a test.
type PowerdMode int

const (
	// DisablePowerd indicates that powerd should be disabled.
	DisablePowerd PowerdMode = iota
	// DoNotChangePowerd indicates that powerd should be left in the same state.
	DoNotChangePowerd
)

// BatteryDischargeMode what setup is needed for a test
type BatteryDischargeMode interface {
	// fulfill implements the indicated battery discharge behavior.
	fulfill(ctx context.Context, s *Setup)
}

// basicBatteryDischargeMode is for battery discharge modes representable as
// constant values.
type basicBatteryDischargeMode int

const (
	// NoBatteryDischarge option requests setup not to try
	// forcing discharge of battery
	NoBatteryDischarge basicBatteryDischargeMode = iota
	// ForceBatteryDischarge option requests setup to force
	// discharging battery during a test
	ForceBatteryDischarge
)

func (battery basicBatteryDischargeMode) fulfill(ctx context.Context, s *Setup) {
	if battery == ForceBatteryDischarge {
		s.Add(SetBatteryDischarge(ctx, 2.0))
	}
}

// advancedBatteryDischargeMode is for battery discharge modes constructed
// from parameters.
type advancedBatteryDischargeMode struct{ errp *error }

// TryBatteryDischarge option requests setup to try battery discharge. If the
// given pointer is nil, TryBatteryDischarge returns ForceBatteryDischarge.
// Otherwise, the battery discharge error is omitted from the power test
// setup and reported through the given pointer instead.
func TryBatteryDischarge(errp *error) BatteryDischargeMode {
	if errp == nil {
		return ForceBatteryDischarge
	}
	return advancedBatteryDischargeMode{errp: errp}
}

func (battery advancedBatteryDischargeMode) fulfill(ctx context.Context, s *Setup) {
	var cleanup CleanupCallback
	cleanup, *battery.errp = SetBatteryDischarge(ctx, 2.0)
	s.Add(cleanup, nil)
}

// UpdateEngineMode indicates what update engine setup is needed for a test.
type UpdateEngineMode int

const (
	// DisableUpdateEngine indicates that update engine should be disabled.
	DisableUpdateEngine UpdateEngineMode = iota
	// DoNotChangeUpdateEngine indicates that update engine should be left in the same state.
	DoNotChangeUpdateEngine
)

// VNCMode indicates what VNC setup is needed for a test.
type VNCMode int

const (
	// DisableVNC indicates that vnc should be disabled.
	DisableVNC VNCMode = iota
	// DoNotChangeVNC indicates that vnc should be left in the same state.
	DoNotChangeVNC
)

// DPTFMode indicates what  DPTF setup is needed for a test.
type DPTFMode int

const (
	// DisableDPTF indicates that dptf should be disabled.
	DisableDPTF DPTFMode = iota
	// DoNotChangeDPTF indicates that dptf should be left in the same state.
	DoNotChangeDPTF
)

// BacklightMode indicates what backlight setup is needed for a test.
type BacklightMode int

const (
	// SetBacklight indicates that back light should be disabled.
	SetBacklight BacklightMode = iota
	// DoNotChangeBacklight indicates that back light should be left in the same state.
	DoNotChangeBacklight
)

// KbBrightnessMode indicates what keyboard brightness setup is needed for a test.
type KbBrightnessMode int

const (
	// SetKbBrightness indicates that back light should be disabled.
	SetKbBrightness KbBrightnessMode = iota
	// DoNotChangeKbBrightness indicates that back light should be left in the same state.
	DoNotChangeKbBrightness
)

// WifiInterfacesMode describes how to setup WiFi interfaces for a test.
type WifiInterfacesMode int

const (
	// DoNotChangeWifiInterfaces indicates that WiFi interfaces should be left in the same state.
	DoNotChangeWifiInterfaces WifiInterfacesMode = iota
	// DisableWifiInterfaces indicates that WiFi interfaces should be disabled.
	DisableWifiInterfaces
)

// NightLightMode what setup is needed for a test.
type NightLightMode int

const (
	// DoNotDisableNightLight indicates that Night Light should be left in the same state.
	DoNotDisableNightLight NightLightMode = iota
	// DisableNightLight indicates that Night Light should be disabled.
	DisableNightLight
)

// AudioMode indicates what audio setup is needed for a test.
type AudioMode int

const (
	// Mute indicates that audio should be muted.
	Mute AudioMode = iota
	// DoNotChangeAudio indicates that audio should be left in the same state.
	DoNotChangeAudio
)

// BluetoothMode indicates what Bluetooth setup is needed for a test.
type BluetoothMode int

const (
	// DisableBluetoothInterfaces indicates that bluetooth should be disabled.
	DisableBluetoothInterfaces BluetoothMode = iota
	// DoNotChangeBluetooth indicates that bluetooth should be left in the same state.
	DoNotChangeBluetooth
)

// PowerTestOptions describes how to set up a power test.
type PowerTestOptions struct {
	// The default value of the following options is not to perform any changes.
	Battery    BatteryDischargeMode
	Wifi       WifiInterfacesMode
	NightLight NightLightMode

	// The default value of the following options is to perform the actions.
	Powerd             PowerdMode
	UpdateEngine       UpdateEngineMode
	VNC                VNCMode
	DPTF               DPTFMode
	Backlight          BacklightMode
	KeyboardBrightness KbBrightnessMode
	Audio              AudioMode
	Bluetooth          BluetoothMode
}

// PowerTest configures a DUT to run a power test by disabling features that add
// noise, and consistently configuring components that change power draw.
func PowerTest(ctx context.Context, c *chrome.TestConn, options PowerTestOptions) (CleanupCallback, error) {
	return Nested(ctx, "power test", func(s *Setup) error {
		if options.Powerd == DisablePowerd {
			s.Add(DisableService(ctx, "powerd"))
		}
		if options.UpdateEngine == DisableUpdateEngine {
			s.Add(DisableService(ctx, "update-engine"))
		}
		if options.VNC == DisableVNC {
			s.Add(DisableServiceIfExists(ctx, "vnc"))
		}
		if options.DPTF == DisableDPTF {
			s.Add(DisableServiceIfExists(ctx, "dptf"))
		}
		if options.Backlight == SetBacklight {
			s.Add(SetBacklightLux(ctx, 150))
		}
		if options.KeyboardBrightness == SetKbBrightness {
			s.Add(SetKeyboardBrightness(ctx, 24))
		}
		if options.Audio == Mute {
			s.Add(MuteAudio(ctx))
		}
		if options.Wifi == DisableWifiInterfaces {
			s.Add(DisableWiFiAdaptors(ctx))
		}
		if options.Battery != nil {
			options.Battery.fulfill(ctx, s)
		}
		if options.Bluetooth == DisableBluetoothInterfaces {
			s.Add(DisableBluetooth(ctx))
		}
		if options.NightLight == DisableNightLight {
			s.Add(TurnOffNightLight(ctx, c))
		}
		return nil
	})
}
