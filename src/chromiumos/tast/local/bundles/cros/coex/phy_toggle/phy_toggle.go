// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package phy_toggle

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bluetooth"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/network/iw"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

// ChangeBluetooth changes the bluetooth setting
func ChangeBluetooth(ctx context.Context, mode string, req string) error {
	cr, err := chrome.New(
		ctx,
		chrome.NoLogin(),
		chrome.LoadSigninProfileExtension(req),
		chrome.KeepState(),
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
	params = ui.FindParams{
		ClassName: "CollapseButton",
	}
	if err := ui.WaitUntilExists(ctx, tLoginConn, params, 10*time.Second); err != nil {
		return errors.Wrap(err, "failed to wait for system tray to open")
	}
	if err := testing.Sleep(ctx, pauseDuration); err != nil {
		return errors.Wrap(err, "failed to sleep")
	}

	// Find the collapse button view bounds.
	bluetoothButton, err := ui.Find(ctx, tLoginConn, ui.FindParams{Name: fmt.Sprintf("Toggle Bluetooth. Bluetooth is %s", mode), ClassName: "FeaturePodIconButton"})
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

func WifiStatus(ctx context.Context) (bool, error) {
	iwr := iw.NewLocalRunner()
	// Get WiFi interface.
	manager, err := shill.NewManager(ctx)
	if err != nil {
		return false, errors.Wrap(err, "failed to create shill manager proxy")
	}

	// GetWifiInterface returns the wireless device interface name (e.g. wlan0), or returns an error on failure.
	_, err = shill.WifiInterface(ctx, manager, 5*time.Second)
	if err != nil {
		return false, nil
	}

	res, err := iwr.ListPhys(ctx)
	if err != nil {
		return false, errors.Wrap(err, "failed to run ListPhys")
	}
	if len(res) == 0 {
		return false, nil
	}
	return true, nil
}

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

func AssertIfUp(ctx context.Context) error {
	res, err := WifiStatus(ctx)
	if err != nil {
		return errors.Wrap(err, "could not query wifi status")
	}
	if !res {
		return errors.New("WiFi not up.")
	}
	res, err = BluetoothStatus(ctx)
	if err != nil {
		return errors.Wrap(err, "could not query bluetooth status")
	}
	if !res {
		return errors.New("Bluetooth not up.")
	}
	return nil
}
func BringIfUp(ctx context.Context, req string) error {
	res, err := WifiStatus(ctx)
	if err != nil {
		return errors.Wrap(err, "could not query wifi status")
	}
	if !res {
		// TODO
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
