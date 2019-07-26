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
		Contacts:     []string{"tetsui@chromium.org", "arc-framework@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Data:         []string{"ArcGamepadTest.apk"},
		Pre:          arc.Booted(),
		Timeout:      4 * time.Minute,
	})
}

type inputDevice struct {
	DeviceID     int    `json:"device_id"`
	ProductID    uint16 `json:"product_id"`
	VendorID     uint16 `json:"vendor_id"`
	Name         string `json:"name"`
	MotionRanges []struct {
		Axis       int     `json:"axis"`
		Flat       float64 `json:"flat"`
		Fuzz       float64 `json:"fuzz"`
		Max        float64 `json:"max"`
		Min        float64 `json:"min"`
		Resolution float64 `json:"resolution"`
		Source     int     `json:"source"`
	} `json:"motion_ranges"`
}

func (d *inputDevice) IsGamepad(gp *input.GamepadEventWriter) bool {
	if d.ProductID != gp.ProductID() || d.VendorID != gp.VendorID() || d.Name != gp.DeviceName() {
		return false
	}
	/*
		if len(d.MotionRanges) != len(gp.Axes()) {
			return false
		}
	*/
	return true
}

type keyEvent struct {
	Action   string `json:"action"`
	KeyCode  string `json:"key_code"`
	DeviceID int    `json:"device_id"`
}

type motionEvent struct {
	Action string             `json:"action"`
	Axes   map[string]float64 `json:"axes"`
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

func getKeyEvents(ctx context.Context, d *ui.Device) ([]keyEvent, error) {
	view := d.Object(ui.ID("org.chromium.arc.testapp.gamepad:id/key_events"))
	text, err := view.GetText(ctx)
	if err != nil {
		return nil, err
	}

	var events []keyEvent
	if err := json.Unmarshal([]byte(text), &events); err != nil {
		return nil, err
	}
	return events, nil
}

func getMotionEvent(ctx context.Context, d *ui.Device) (*motionEvent, error) {
	view := d.Object(ui.ID("org.chromium.arc.testapp.gamepad:id/motion_event"))
	text, err := view.GetText(ctx)
	if err != nil {
		return nil, err
	}

	var event motionEvent
	if err := json.Unmarshal([]byte(text), &event); err != nil {
		return nil, err
	}
	return &event, nil
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
	defer func() {
		if gp != nil {
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

	s.Log("Checking the device connection")
	var actualDevice inputDevice
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		devices, err := getInputDevices(ctx, d)
		if err != nil {
			return err
		} else if len(devices) != 1 {
			return errors.Errorf("unexpected number of gamepad devices: got %v; want 1",
				len(devices))
		}
		actualDevice = devices[0]
		return nil
	}, nil); err != nil {
		s.Fatal("Cannot get the gamepad device: ", err)
	}

	// DeviceID may change at runtime.
	if !actualDevice.IsGamepad(gp) {
		s.Fatal("Unexpected device information: got ", actualDevice)
	}

	s.Log("Pressing buttons")
	if err := gp.TapButton(ctx, input.BTN_EAST); err != nil {
		s.Fatal("Failed to press button: ", err)
	}
	if err := gp.TapButton(ctx, input.BTN_SOUTH); err != nil {
		s.Fatal("Failed to press button: ", err)
	}

	const (
		ActionDown     = "ACTION_DOWN"
		ActionUp       = "ACTION_UP"
		KeycodeButtonA = "KEYCODE_BUTTON_A"
		KeycodeButtonX = "KEYCODE_BUTTON_X"
	)

	expectedEvents := []keyEvent{
		{Action: ActionDown, KeyCode: KeycodeButtonA},
		{Action: ActionUp, KeyCode: KeycodeButtonA},
		{Action: ActionDown, KeyCode: KeycodeButtonX},
		{Action: ActionUp, KeyCode: KeycodeButtonX}}

	s.Log("Checking the generated gamepad events")
	var actualEvents []keyEvent
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var err error
		if actualEvents, err = getKeyEvents(ctx, d); err != nil {
			return err
		} else if len(actualEvents) != len(expectedEvents) {
			return errors.Errorf("unexpected number of gamepad events: got %d; want %d",
				len(actualEvents), len(expectedEvents))
		}
		return nil
	}, nil); err != nil {
		s.Fatal("Failed to get gamepad events: ", err)
	}

	for i, expected := range expectedEvents {
		// DeviceID may change at runtime.
		expected.DeviceID = actualDevice.DeviceID
		if expected != actualEvents[i] {
			s.Fatalf("Unexpected gamepad event: got %v; want %v", actualEvents[i], expected)
		}
	}

	s.Log("Moving the analog stick")
	if err := gp.MoveAxis(ctx, input.ABS_X, gp.Axes()[input.AxisCode(input.ABS_X)].Maximum); err != nil {
		s.Fatal("Failed to press button: ", err)
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if event, err := getMotionEvent(ctx, d); err != nil {
			return err
		} else if val, ok := event.Axes["AXIS_X"]; !ok || val != 1.0 {
			return errors.Errorf("unexpected AXIS_X: got %f", val)
		}
		return nil
	}, nil); err != nil {
		s.Fatal("Failed to get gamepad events: ", err)
	}

	if err := gp.MoveAxis(ctx, input.ABS_X, gp.Axes()[input.AxisCode(input.ABS_X)].Minimum); err != nil {
		s.Fatal("Failed to press button: ", err)
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if event, err := getMotionEvent(ctx, d); err != nil {
			return err
		} else if val, ok := event.Axes["AXIS_X"]; !ok || val != -1.0 {
			return errors.Errorf("unexpected AXIS_X: got %f", val)
		}
		return nil
	}, nil); err != nil {
		s.Fatal("Failed to get gamepad events: ", err)
	}

	s.Log("Disconnecting the gamepad")
	if err := gp.Close(); err != nil {
		s.Fatal("Failed to close the gamepad: ", err)
	}
	gp = nil

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if devices, err := getInputDevices(ctx, d); err != nil {
			return err
		} else if len(devices) > 0 {
			return errors.Errorf("the gamepad device still exist: %+v", devices)
		}
		return nil
	}, nil); err != nil {
		s.Fatal("Failed to disconnect the gamepad: ", err)
	}
}
