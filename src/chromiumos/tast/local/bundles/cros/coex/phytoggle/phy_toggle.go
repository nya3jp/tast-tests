// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

//Package phytoggle contains functions that actuate the device's wireless interfaces through the UI.
package phytoggle

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bluetooth"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

// ChangeBluetooth changes the bluetooth setting
func ChangeBluetooth(ctx context.Context, mode, req string) error {
	cr, err := chrome.New(
		ctx,
		chrome.KeepState(),
		chrome.NoLogin(),
		chrome.LoadSigninProfileExtension(req),
	)
	if err != nil {
		return errors.Wrap(err, "failed to start Chrome")
	}
	defer cr.Close(ctx)
	tLoginConn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create login test API connection")
	}
	defer tLoginConn.Close()
	const pauseDuration = 5 * time.Second
	if err := testing.Sleep(ctx, pauseDuration); err != nil {
		return errors.Wrap(err, "failed to sleep")
	}

	// Find and click the StatusArea via UI. Clicking it opens the Ubertray.
	params := ui.FindParams{
		ClassName: "ash/StatusAreaWidgetDelegate",
	}
	statusArea, err := ui.FindWithTimeout(ctx, tLoginConn, params, 10*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed to find the status area (time, battery, etc.)")
	}
	defer statusArea.Release(ctx)
	if err := statusArea.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click status area")
	}

	// Confirm that the system tray is open by checking for the "CollapseButton".
	params = ui.FindParams{Name: fmt.Sprintf("Toggle Bluetooth. Bluetooth is %s", mode), ClassName: "FeaturePodIconButton"}
	if err := ui.WaitUntilExists(ctx, tLoginConn, params, 10*time.Second); err != nil {
		return errors.Wrap(err, "failed to wait for bluetooth button to appear")
	}

	// Find the bluetooth button view bounds.
	bluetoothButton, err := ui.Find(ctx, tLoginConn, params)
	if err != nil {
		if err != nil {
			return errors.Wrap(err, "failed to find bluetooth button")
		}
	}
	defer bluetoothButton.Release(ctx)
	if err := bluetoothButton.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click bluetooth button")
	}
	return nil
}

// ChangeWifi changes the bluetooth setting
func ChangeWifi(ctx context.Context, mode, req string) error {
	cr, err := chrome.New(
		ctx,
		chrome.KeepState(),
		chrome.NoLogin(),
		chrome.LoadSigninProfileExtension(req),
	)
	if err != nil {
		return errors.Wrap(err, "failed to start Chrome")
	}
	defer cr.Close(ctx)
	tLoginConn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create login test API connection")
	}
	defer tLoginConn.Close()
	const pauseDuration = 5 * time.Second
	if err := testing.Sleep(ctx, pauseDuration); err != nil {
		return errors.Wrap(err, "failed to sleep")
	}

	// Find and click the StatusArea via UI. Clicking it opens the Ubertray.
	params := ui.FindParams{
		ClassName: "ash/StatusAreaWidgetDelegate",
	}
	statusArea, err := ui.FindWithTimeout(ctx, tLoginConn, params, 10*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed to find the status area (time, battery, etc.)")
	}
	defer statusArea.Release(ctx)
	if err := statusArea.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click status area")
	}

	// Confirm that the system tray is open by checking for the "CollapseButton".
	params = ui.FindParams{Name: "Toggle network connection. Connected to Ethernet", ClassName: "FeaturePodIconButton"}
	if err := ui.WaitUntilExists(ctx, tLoginConn, params, 10*time.Second); err != nil {
		return errors.Wrap(err, "failed to wait for system tray to open")
	}
	// Find the ethernet button view bounds.
	ethernetButton, err := ui.Find(ctx, tLoginConn, params)
	if err != nil {
		if err != nil {
			return errors.Wrap(err, "failed to find ethernetbutton")
		}
	}

	defer ethernetButton.Release(ctx)
	if err := ethernetButton.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click ethernet button")
	}

	var state ui.CheckedState
	if mode == "on" {
		state = "true"
	} else {
		state = "false"
	}
	params = ui.FindParams{Name: "Wi-Fi", ClassName: "ToggleButton"}
	if err := ui.WaitUntilExists(ctx, tLoginConn, params, 10*time.Second); err != nil {
		return errors.Wrap(err, "failed to wait for wifi switch to appear")
	}
	wifiButton, err := ui.Find(ctx, tLoginConn, params)
	if err != nil {
		if err != nil {
			return errors.Wrap(err, "failed to find WiFi switch")
		}
	}
	if state != wifiButton.Checked {
		return errors.Errorf("unexpected Wifi status, expected %d, but got %d", state, wifiButton.Checked)
	}
	defer wifiButton.Release(ctx)
	if err := wifiButton.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click wifi button")
	}
	return nil
}

// BluetoothStatus returns the status of the bluetooth interface
func BluetoothStatus(ctx context.Context) (bool, error) {
	adapters, err := bluetooth.Adapters(ctx)
	if err != nil {
		return false, errors.Wrap(err, "unable to get Bluetooth adapters")
	}

	if len(adapters) != 1 {
		return false, nil
	}
	adapter := adapters[0]
	return adapter.Powered(ctx)
}

// WifiStatus returns the status of the Wifi interface
func WifiStatus(ctx context.Context) (bool, error) {
	m, err := shill.NewManager(ctx)
	if err != nil {
		return false, errors.Wrap(err, "failed to get shill manager")
	}
	watcher, err := m.CreateWatcher(ctx)
	if err != nil {
		return false, errors.Wrap(err, "failed to create a PropertiesWatcher")
	}
	defer watcher.Close(ctx)

	prop, err := m.GetProperties(ctx)
	technologies, err := prop.GetStrings(shillconst.ManagerPropertyEnabledTechnologies)
	if err != nil {
		return false, errors.Wrap(err, "failed to get enabled technologies")
	}

	found := false
	for _, t := range technologies {
		if t == string(shill.TechnologyWifi) {
			found = true
			break
		}
	}
	if found {
		return true, nil
	}
	return false, nil
}

//AssertPhysUp checks to make sure that both Bluetooth and Wifi are enabled
func AssertPhysUp(ctx context.Context) error {
	res, err := WifiStatus(ctx)
	if err != nil {
		return errors.Wrap(err, "could not query wifi status")
	}
	if !res {
		return errors.New("wifi not up")
	}
	res, err = BluetoothStatus(ctx)
	if err != nil {
		return errors.Wrap(err, "could not query bluetooth status")
	}
	if !res {
		return errors.New("bluetooth not up")
	}
	return nil
}

//BringPhysUp reenables Bluetooth and Wifi should any or both interfaces be disabled
func BringPhysUp(ctx context.Context, req string) error {
	res, err := WifiStatus(ctx)
	if err != nil {
		return errors.Wrap(err, "could not query wifi status")
	}
	if !res {
		if err := ChangeWifi(ctx, "off", req); err != nil {
			return errors.Wrap(err, "could not turn on bluetooth")
		}
	}
	res, err = BluetoothStatus(ctx)
	if err != nil {
		return errors.Wrap(err, "could not query bluetooth status")
	}
	if !res {
		if err := ChangeBluetooth(ctx, "off", req); err != nil {
			return errors.Wrap(err, "could not turn on bluetooth")
		}
	}
	return nil
}
