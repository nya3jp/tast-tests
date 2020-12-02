// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"
	"regexp"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/bundles/cros/ui/settingsapp"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/quicksettings"
	"chromiumos/tast/testing"
)

// waitInterval defines the time duration between actions.
const waitInterval = 2 * time.Second

// Bluetooth implements bluetooth control.
type Bluetooth struct{}

// New creates a Bluetooth instance.  It also enables bluetooth feature through quick settings if disabled.
func New(ctx context.Context, tconn *chrome.TestConn) (*Bluetooth, error) {
	if err := quicksettings.ToggleSetting(ctx, tconn, quicksettings.SettingPodBluetooth, true); err != nil {
		return &Bluetooth{}, err
	}
	return &Bluetooth{}, nil
}

// Close disable bluetooth.
func (b *Bluetooth) Close(ctx context.Context, tconn *chrome.TestConn) error {
	testing.ContextLog(ctx, "bluetooth closed")
	if err := quicksettings.ToggleSetting(ctx, tconn, quicksettings.SettingPodBluetooth, false); err != nil {
		return err
	}
	return nil
}

func newDeviceFindParams(name string) ui.FindParams {
	pattern := regexp.QuoteMeta(name)
	return ui.FindParams{
		Role:       ui.RoleTypeButton,
		ClassName:  "list-item",
		Attributes: map[string]interface{}{"name": regexp.MustCompile(pattern)},
	}
}

// isConnectedState finds the Bluetooth device listed in the Settings app, checking
// the connection state of the device.
func isConnectedState(ctx context.Context, app *settingsapp.SettingsApp, params ui.FindParams) (bool, error) {
	dev, err := app.Root.Descendant(ctx, params)
	if err != nil {
		return false, err
	}
	defer dev.Release(ctx)

	label, err := dev.Descendant(ctx, ui.FindParams{
		Attributes: map[string]interface{}{"name": regexp.MustCompile("[Cc]onnect")},
	})
	if err != nil {
		return false, err
	}
	defer label.Release(ctx)
	return label.Name == "Connected", nil
}

// openSettings launches the Settings app, navigating to the Bluetooth subpage.
func (b *Bluetooth) openSettings(ctx context.Context, tconn *chrome.TestConn) (*settingsapp.SettingsApp, error) {
	app, err := settingsapp.Launch(ctx, tconn)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open app: %s", "Settings")
	}
	defer func() {
		if err != nil {
			// instead of closing the app, we keep the window for trouble shooting,
			// for example, taking screenshot.
			app.Root.Release(ctx)
		}
	}()

	if err = app.NavigateTo(ctx, settingsapp.Bluetooth); err != nil {
		return nil, errors.Wrapf(err, "failed to navigate URL: %s", settingsapp.Bluetooth)
	}
	if err := ui.WaitForLocationChangeCompleted(ctx, tconn); err != nil {
		return nil, errors.Wrap(err, "failed to wait for animation finished")
	}
	if err = app.OpenSubpage(ctx, settingsapp.Bluetooth); err != nil {
		return nil, errors.Wrapf(err, "failed to click %q in Chrome OS app", "Bluetooth")
	}
	return app, nil
}

// Connect connects to a Bluetooth device by the given name.
func (b *Bluetooth) Connect(ctx context.Context, tconn *chrome.TestConn, deviceName string) error {
	if deviceName == "" {
		return status.Error(codes.InvalidArgument, "device name is empty")
	}
	testing.ContextLogf(ctx, "bluetooth: connect %q via Settings App", deviceName)

	app, err := b.openSettings(ctx, tconn)
	if err != nil {
		return err
	}
	defer app.Close(ctx)

	params := newDeviceFindParams(deviceName)
	if err = cuj.WaitAndClickDescendant(ctx, app.Root, params, 30*time.Second); err != nil {
		return errors.Wrapf(err, "failed to click %q in Chrome OS app", deviceName)
	}

	// wait the state to be "Connected"
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if connected, err := isConnectedState(ctx, app, params); err != nil {
			return err
		} else if !connected {
			return errors.Wrap(err, "connection state is not connected")
		}
		return nil
	}, &testing.PollOptions{Timeout: 15 * time.Second}); err != nil {
		return errors.Wrapf(err, "bluetooth device (%q) connect failed", deviceName)
	}
	return nil
}

// IsConnected checks if the named device is connected.
func (b *Bluetooth) IsConnected(ctx context.Context, tconn *chrome.TestConn, deviceName string) (bool, error) {
	if deviceName == "" {
		return false, status.Error(codes.InvalidArgument, "device name is empty")
	}
	testing.ContextLogf(ctx, "bluetooth: check %q connectivity", deviceName)

	app, err := b.openSettings(ctx, tconn)
	if err != nil {
		return false, err
	}
	defer app.Close(ctx)

	params := newDeviceFindParams(deviceName)

	// wait the state to be "Connected"
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if connected, err := isConnectedState(ctx, app, params); err != nil {
			return err
		} else if !connected {
			return errors.Wrap(err, "connection state is not connected")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return false, errors.Wrapf(err, "failed to get bluetooth device status. deviceName=%q", deviceName)
	}
	return true, nil
}
