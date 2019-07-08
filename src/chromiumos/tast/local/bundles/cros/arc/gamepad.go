// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"encoding/json"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Gamepad,
		Desc:         "Checks gamepad support works on Android",
		Contacts:     []string{"tetsui@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome"},
		Data:         []string{"ArcGamepadTest.apk"},
		Pre:          arc.Booted(),
		Timeout:      4 * time.Minute,
	})
}

type inputDevice struct {
	ProductID int    `json:"product_id"`
	VendorID  int    `json:"vendor_id"`
	Name      string `json:"name"`
}

func getInputDevices(ctx context.Context, d *ui.Device) ([]inputDevice, error) {
	view := d.Object(ui.ID("org.chromium.arc.testapp.gamepad:id/device_status"))
	text, err := view.GetText(ctx)
	if err != nil {
		return nil, err
	}

	var devices []inputDevice
	if err := json.Unmarshal([]byte(text), &devices); err != nil {
		return nil, err
	}
	return devices, nil
}

func Gamepad(ctx context.Context, s *testing.State) {
	a := s.PreValue().(arc.PreData).ARC
	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close()

	gp, err := input.Gamepad(ctx)
	if err != nil {
		s.Fatal("Failed to create a gamepad: ", err)
	}
	isClosed := false
	defer func() {
		if !isClosed {
			gp.Close()
		}
	}()

	s.Log("Created a virtual gamepad device ", gp.Device())

	const (
		apk = "ArcGamepadTest.apk"
		pkg = "org.chromium.arc.testapp.gamepad"
		cls = "org.chromium.arc.testapp.gamepad.MainActivity"
	)

	s.Log("Installing app")
	if err := a.Install(ctx, s.DataPath(apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	s.Log("Starting app")
	if err := a.Command(ctx, "am", "start", "-W", pkg+"/"+cls).Run(); err != nil {
		s.Fatal("Failed starting app: ", err)
	}

	expectedDevice := inputDevice{ProductID: 0x09cc, VendorID: 0x054c, Name: "Sony Interactive Entertainment Wireless Controller"}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		devices, err := getInputDevices(ctx, d)
		if err != nil {
			return err
		}
		for _, device := range devices {
			if device == expectedDevice {
				return nil
			}
		}
		return errors.New("cannot find the device")
	}, nil); err != nil {
		s.Fatal("Failed to confirm the device connected: ", err)
	}

	if err := gp.PressButton(ctx, input.BTN_EAST); err != nil {
		s.Fatal("Failed to press button: ", err)
	}

	if err := gp.ReleaseButton(ctx, input.BTN_EAST); err != nil {
		s.Fatal("Failed to release button: ", err)
	}

	if false {
		if err := gp.MoveAxis(ctx, input.ABS_X, 200); err != nil {
			s.Fatal("Failed to move axis: ", err)
		}
		testing.Sleep(ctx, 15*time.Second)
		if err := gp.MoveAxis(ctx, input.ABS_X, -200); err != nil {
			s.Fatal("Failed to move axis: ", err)
		}
	}

	isClosed = true
	if err := gp.Close(); err != nil {
		s.Fatal("Failed to close gamepad: ", err)
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		devices, err := getInputDevices(ctx, d)
		if err != nil {
			return err
		}
		for _, device := range devices {
			if device == expectedDevice {
				return errors.New("still connected")
			}
		}
		return nil
	}, nil); err != nil {
		s.Fatal("Failed to confirm the device disconnected: ", err)
	}

}
