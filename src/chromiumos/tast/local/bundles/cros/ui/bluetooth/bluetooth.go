// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"
	"regexp"
	"time"

	"github.com/golang/protobuf/ptypes/wrappers"
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

// Bluetooth struct
type Bluetooth struct{}

// New initail bluetooth instance and enable bluetooth.
// The returned App instance must be closed when the test is finished.
func New(ctx context.Context, tconn *chrome.TestConn) (*Bluetooth, error) {
	if err := quicksettings.ToggleSetting(ctx, tconn, quicksettings.SettingPodBluetooth, true); err != nil {
		return &Bluetooth{}, err
	}
	return &Bluetooth{}, nil
}

// Close closes the App and the associated connection.
func (b *Bluetooth) Close(ctx context.Context, tconn *chrome.TestConn) error {
	testing.ContextLog(ctx, "bluetooth closed")
	if err := quicksettings.ToggleSetting(ctx, tconn, quicksettings.SettingPodBluetooth, false); err != nil {
		return err
	}
	return nil
}

func newDeviceFindParams(name string) ui.FindParams {
	return ui.FindParams{
		Role:       ui.RoleTypeButton,
		ClassName:  "list-item",
		Attributes: map[string]interface{}{"name": regexp.MustCompile(name)},
	}
}

// connectionState finds the Bluetooth device listed in the Settings app, checking
// the connection state of the device.
func connectionState(ctx context.Context, app *settingsapp.SettingsApp, params ui.FindParams) (string, error) {
	dev, err := app.Root.Descendant(ctx, params)
	if err != nil {
		return "", err
	}
	defer dev.Release(ctx)

	label, err := dev.Descendant(ctx, ui.FindParams{
		Attributes: map[string]interface{}{"name": regexp.MustCompile("[Cc]onnect")},
	})
	if err != nil {
		return "", err
	}
	defer label.Release(ctx)
	return label.Name, nil
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

	testing.Sleep(ctx, waitInterval)
	if err = app.NavigateTo(ctx, settingsapp.Bluetooth); err != nil {
		return nil, errors.Wrapf(err, "failed to navigate URL: %s", settingsapp.Bluetooth)
	}

	testing.Sleep(ctx, waitInterval)
	if err = app.OpenSubpage(ctx, settingsapp.Bluetooth); err != nil {
		return nil,
			errors.Wrapf(err, "failed to click %s in Chrome OS app", "Bluetooth")
	}
	return app, nil
}

// Connect connects to a Bluetooth device by the given name.
func (b *Bluetooth) Connect(ctx context.Context, tconn *chrome.TestConn, name *wrappers.StringValue) error {
	if name.Value == "" {
		return status.Error(codes.InvalidArgument, "device name is empty")
	}
	testing.ContextLogf(ctx, "bluetooth: connect %s via Settings App", name.Value)

	app, err := b.openSettings(ctx, tconn)
	if err != nil {
		return err
	}
	defer app.Close(ctx)

	testing.Sleep(ctx, waitInterval)
	params := newDeviceFindParams(name.Value)
	if err = cuj.WaitAndClickDescendant(ctx, app.Root, params, 30*time.Second); err != nil {
		return errors.Wrapf(err, "failed to click %s in Chrome OS app", name)
	}

	// wait the state to be "Connected"
	err = testing.Poll(ctx, func(c context.Context) error {
		state, err := connectionState(ctx, app, params)
		if err != nil {
			return err
		}
		if state != "Connected" {
			return errors.Wrapf(err, "connection state is %s", state)
		}
		return nil
	}, &testing.PollOptions{Timeout: 15 * time.Second})
	if err != nil {
		return errors.Wrapf(err, "bluetooth device (%v) connect failed", name.Value)
	}
	return nil
}

// IsConnected checks if the named device is connected.
func (b *Bluetooth) IsConnected(ctx context.Context, tconn *chrome.TestConn, name *wrappers.StringValue) (*wrappers.BoolValue, error) {
	if name.Value == "" {
		return nil, status.Error(codes.InvalidArgument, "device name is empty")
	}
	testing.ContextLogf(ctx, "bluetooth: check %s connectivity", name.Value)

	app, err := b.openSettings(ctx, tconn)
	if err != nil {
		return nil, err
	}
	defer app.Close(ctx)

	testing.Sleep(ctx, waitInterval)
	params := newDeviceFindParams(name.Value)
	state, err := connectionState(ctx, app, params)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get bluetooth device status. deviceName=%v", name.Value)
	}
	return &wrappers.BoolValue{Value: state == "Connected"}, nil
}
