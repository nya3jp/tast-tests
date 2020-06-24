// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"encoding/json"
	"math"

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
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome"},
		Pre:          arc.Booted(),
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

type inputDevice struct {
	DeviceID     int    `json:"device_id"`
	ProductID    uint16 `json:"product_id"`
	VendorID     uint16 `json:"vendor_id"`
	Name         string `json:"name"`
	MotionRanges map[string]struct {
		Flat       float64 `json:"flat"`
		Fuzz       float64 `json:"fuzz"`
		Max        float64 `json:"max"`
		Min        float64 `json:"min"`
		Resolution float64 `json:"resolution"`
		Source     int     `json:"source"`
	} `json:"motion_ranges"`
}

// verifyGamepadDeviceInfo confirms the gamepad's InputDevice information is correct.
func verifyGamepadDeviceInfo(s *testing.State, gp *input.GamepadEventWriter, d *inputDevice) {
	// DeviceID may change at runtime.
	if d.ProductID != gp.ProductID() {
		s.Errorf("product ID doesn't match: got %v; want %v", d.ProductID, gp.ProductID())
	}
	if d.VendorID != gp.VendorID() {
		s.Errorf("vendor ID doesn't match: got %v; want %v", d.VendorID, gp.VendorID())
	}

	isCenteredAxis := map[string]bool{
		"AXIS_X":     true,
		"AXIS_Y":     true,
		"AXIS_Z":     true,
		"AXIS_RZ":    true,
		"AXIS_HAT_X": true,
		"AXIS_HAT_Y": true,
	}
	gamepadMapping := map[input.EventCode]string{
		input.ABS_X:     "AXIS_X",
		input.ABS_Y:     "AXIS_Y",
		input.ABS_RX:    "AXIS_Z",
		input.ABS_RY:    "AXIS_RZ",
		input.ABS_HAT0X: "AXIS_HAT_X",
		input.ABS_HAT0Y: "AXIS_HAT_Y",
		input.ABS_Z:     "AXIS_LTRIGGER",
		input.ABS_RZ:    "AXIS_RTRIGGER",
	}

	almostEqual := func(a, b float64) bool {
		return math.Abs(a-b) <= 1e-5
	}

	for code, info := range gp.Axes() {
		axisName, ok := gamepadMapping[code]
		if !ok {
			continue
		}
		expectedMin := 0.0
		expectedMax := 1.0
		if isCenteredAxis[axisName] {
			expectedMin = -1.0
			expectedMax = 1.0
		}
		scale := (expectedMax - expectedMin) / float64(info.Maximum-info.Minimum)
		expectedFuzz := float64(info.Fuzz) * scale
		expectedFlat := float64(info.Flat) * scale

		motionRange, ok := d.MotionRanges[axisName]
		if !ok {
			s.Errorf("gamepad axis %s not found", axisName)
		}
		if !almostEqual(expectedMin, motionRange.Min) {
			s.Errorf("min does not match for %s: got %f; want %f", axisName, motionRange.Min, expectedMin)
		}
		if !almostEqual(expectedMax, motionRange.Max) {
			s.Errorf("max does not match for %s: got %f; want %f", axisName, motionRange.Max, expectedMax)
		}
		if !almostEqual(expectedFuzz, motionRange.Fuzz) {
			s.Errorf("fuzz does not match for %s: got %f; want %f", axisName, motionRange.Fuzz, expectedFuzz)
		}
		if !almostEqual(expectedFlat, motionRange.Flat) {
			s.Errorf("flat does not match for %s: got %f; want %f", axisName, motionRange.Flat, expectedFlat)
		}
	}
	return
}

type gamepadKeyEvent struct {
	Action   string `json:"action"`
	KeyCode  string `json:"key_code"`
	DeviceID int    `json:"device_id"`
}

type gamepadMotionEvent struct {
	Action string             `json:"action"`
	Axes   map[string]float64 `json:"axes"`
}

// getInputDevices gets the input device information saved in app UI.
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

// getGamepadKeyEvents gets all key events saved in app UI.
func getGamepadKeyEvents(ctx context.Context, d *ui.Device) ([]gamepadKeyEvent, error) {
	view := d.Object(ui.ID("org.chromium.arc.testapp.gamepad:id/key_events"))
	text, err := view.GetText(ctx)
	if err != nil {
		return nil, err
	}

	var events []gamepadKeyEvent
	if err := json.Unmarshal([]byte(text), &events); err != nil {
		return nil, err
	}
	return events, nil
}

// waitForKeyEvents waits until the app receives expected events and the length of received events is same as expected events.
func waitForKeyEvents(ctx context.Context, d *ui.Device, deviceID int, expects []gamepadKeyEvent) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		events, err := getGamepadKeyEvents(ctx, d)
		if err != nil {
			return err
		} else if len(events) != len(expects) {
			return errors.Errorf("unexpected number of gamepad key events: got %d; want %d", len(events), len(expects))
		} else {
			for i, expect := range expects {
				// DeviceID may change at runtime.
				expect.DeviceID = deviceID
				if expect != events[i] {
					return errors.Errorf("unexpected gamepad event: got %v; want %v", events[i], expect)
				}
			}
			return nil
		}
	}, nil)
}

// getGamepadMotionEvent gets the motion event saved in app UI.
func getGamepadMotionEvent(ctx context.Context, d *ui.Device) (*gamepadMotionEvent, error) {
	view := d.Object(ui.ID("org.chromium.arc.testapp.gamepad:id/motion_event"))
	text, err := view.GetText(ctx)
	if err != nil {
		return nil, err
	}

	var event gamepadMotionEvent
	if err := json.Unmarshal([]byte(text), &event); err != nil {
		return nil, err
	}
	return &event, nil
}

// waitForGamepadAxis waits until the app receives expected motion event.
func waitForGamepadAxis(ctx context.Context, d *ui.Device, axis string, expected float64) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		if event, err := getGamepadMotionEvent(ctx, d); err != nil {
			return err
		} else if val, ok := event.Axes[axis]; !ok || val != expected {
			return errors.Errorf("unexpected %s value: got %f; want %f", axis, val, expected)
		}
		return nil
	}, nil)
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
	if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	s.Log("Starting app")
	if err := a.Command(ctx, "am", "start", "-W", pkg+"/"+cls).Run(); err != nil {
		s.Fatal("Failed starting app: ", err)
	}

	s.Log("Checking the device connection")
	var device inputDevice
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		devices, err := getInputDevices(ctx, d)
		if err != nil {
			return err
		} else if len(devices) != 1 {
			return errors.Errorf("unexpected number of gamepad devices: got %v; want 1",
				len(devices))
		}
		device = devices[0]
		return nil
	}, nil); err != nil {
		s.Fatal("Cannot get the gamepad device: ", err)
	}

	verifyGamepadDeviceInfo(s, gp, &device)

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
		AxisX          = "AXIS_X"
	)

	expectedEvents := []gamepadKeyEvent{
		{Action: ActionDown, KeyCode: KeycodeButtonA},
		{Action: ActionUp, KeyCode: KeycodeButtonA},
		{Action: ActionDown, KeyCode: KeycodeButtonX},
		{Action: ActionUp, KeyCode: KeycodeButtonX}}

	s.Log("Checking the generated gamepad events")
	if err := waitForKeyEvents(ctx, d, device.DeviceID, expectedEvents); err != nil {
		s.Fatal("Failed to wait for key events: ", err)
	}

	s.Log("Moving the analog stick")
	axis := gp.Axes()[input.ABS_X]
	if err := gp.MoveAxis(ctx, input.ABS_X, axis.Maximum); err != nil {
		s.Fatal("Failed to move axis: ", err)
	}
	if err := waitForGamepadAxis(ctx, d, AxisX, 1.0); err != nil {
		s.Fatal("Failed to wait for axis change: ", err)
	}
	if err := gp.MoveAxis(ctx, input.ABS_X, axis.Minimum); err != nil {
		s.Fatal("Failed to move axis: ", err)
	}
	if err := waitForGamepadAxis(ctx, d, AxisX, -1.0); err != nil {
		s.Fatal("Failed to wait for axis change: ", err)
	}

	// Press button and move axis together, and then release button.
	s.Log("Pressing button and moving the analog stick together")
	injectEvents := []input.GamepadEvent{
		{Et: input.EV_ABS, Ec: input.ABS_X, Val: axis.Maximum},
		{Et: input.EV_KEY, Ec: input.BTN_EAST, Val: 1}}

	if err := gp.PressButtonsAndAxes(ctx, injectEvents); err != nil {
		s.Fatal("Failed to press button and move axis together: ", err)
	}

	// Release button.
	if err := gp.ReleaseButton(ctx, input.BTN_EAST); err != nil {
		s.Fatal("Failed to release button: ", err)
	}

	expectedEvents = append(expectedEvents, gamepadKeyEvent{Action: ActionDown, KeyCode: KeycodeButtonA})
	expectedEvents = append(expectedEvents, gamepadKeyEvent{Action: ActionUp, KeyCode: KeycodeButtonA})

	if err := waitForKeyEvents(ctx, d, device.DeviceID, expectedEvents); err != nil {
		s.Fatal("Failed to wait for key event change: ", err)
	}
	if err := waitForGamepadAxis(ctx, d, AxisX, 1.0); err != nil {
		s.Fatal("Failed to wait for axis change: ", err)
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
