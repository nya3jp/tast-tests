// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"fmt"
	"math"
	"reflect"
	"strings"
	"unsafe"

	"chromiumos/tast/local/bundles/cros/camera/cca"
	"chromiumos/tast/local/bundles/cros/camera/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/testing"
)

type testParameter struct {
	Control      string
	ControlMode  string
	Precondition string
	Tolerance    float64
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIMediaTrackAdvancedControls,
		Desc:         "Opens CCA and verifies the MediaTrack advanced controls",
		Contacts:     []string{"mojahsu@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", caps.BuiltinOrVividCamera},
		Data:         []string{"cca_ui.js"},
		Pre:          chrome.LoggedIn(),
	})
}

func CCAUIMediaTrackAdvancedControls(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tb, err := testutil.NewTestBridge(ctx, cr)
	if err != nil {
		s.Fatal("Failed to construct test bridge: ", err)
	}
	defer tb.TearDown(ctx)

	if err := cca.ClearSavedDirs(ctx, cr); err != nil {
		s.Fatal("Failed to clear saved directory: ", err)
	}

	app, err := cca.New(ctx, cr, []string{s.DataPath("cca_ui.js")}, s.OutDir(), tb)
	if err != nil {
		s.Fatal("Failed to open CCA: ", err)
	}
	defer func(ctx context.Context) {
		if err := app.Close(ctx); err != nil {
			s.Error("Failed to close app: ", err)
		}
	}(ctx)

	numCameras, err := app.GetNumOfCameras(ctx)
	if err != nil {
		s.Fatal("Can't get number of cameras: ", err)
	}

	if numCameras > 1 {
		verifyAdvancedControls(ctx, s, app)
		// Switch camera.
		if err := app.SwitchCamera(ctx); err != nil {
			s.Fatal("Switch camera failed: ", err)
		}
		verifyAdvancedControls(ctx, s, app)
	} else if numCameras == 1 {
		verifyAdvancedControls(ctx, s, app)
	} else {
		s.Fatal("No camera found")
	}
}

func verifyAdvancedControls(ctx context.Context, s *testing.State, app *cca.App) {
	parameters := [...]testParameter{
		{"exposureTime", "exposureMode", "", 1.0},
		{"focusDistance", "focusMode", "", 0.01},
		{"colorTemperature", "whiteBalanceMode", "", 0.0},
		{"exposureCompensation", "", "", 0.0},
		{"iso", "", "\"exposureMode\":\"manual\", \"exposureTime\":333,", 5.0},
		{"brightness", "", "", 0.0},
		{"contrast", "", "", 0.0},
		{"saturation", "", "", 0.0},
		{"sharpness", "", "", 0.0},
		{"pan", "", "", 0.0},
		{"tilt", "", "", 0.0},
		{"zoom", "", "", 0.0},
	}
	capabilities, err := app.GetMediaTrackCapabilities(ctx)
	if err != nil {
		s.Fatal("Can't get capabilities: ", err)
		return
	}
	for _, parameter := range parameters {
		ss := reflect.ValueOf(&capabilities).Elem().FieldByName(strings.Title(parameter.Control))
		if ss.IsNil() {
			continue
		}
		settingRange := *(*cca.MediaSettingsRange)(unsafe.Pointer(ss.Pointer()))
		s.Logf("support %s Range max:%f, min:%f, step:%f", parameter.Control, settingRange.Max, settingRange.Min, settingRange.Step)
		if parameter.ControlMode != "" {
			ss := reflect.ValueOf(&capabilities).Elem().FieldByName(strings.Title(parameter.ControlMode))
			if ss.IsNil() {
				s.Fatal("mode is not supported: " + strings.Title(parameter.ControlMode))
				return
			}
			mode := *(*[]string)(unsafe.Pointer(ss.Pointer()))
			s.Logf("support %s:%s", parameter.ControlMode, mode)
		}
		verifyControlRange(ctx, s, app, parameter, settingRange)
	}
}

func verifyControlRange(ctx context.Context, s *testing.State, app *cca.App, parameter testParameter, settingRange cca.MediaSettingsRange) {
	totalSteps := (settingRange.Max - settingRange.Min) / settingRange.Step
	middleValue := settingRange.Min + settingRange.Step*math.Round(totalSteps/2)
	testValues := []float64{settingRange.Min, middleValue, settingRange.Max}
	// We need to use specified values for color temperatures, because it is translated from white balance mode.
	availableColorTemperatures := []float64{2700, 3000, 5000, 5500, 6500, 7500, 15000}

	if parameter.Control == "colorTemperature" {
		testValues = []float64{}
		for _, value := range availableColorTemperatures {
			if value >= settingRange.Min && value <= settingRange.Max {
				testValues = append(testValues, value)
			}
		}
	}

	for _, value := range testValues {
		var constraint string
		if parameter.ControlMode != "" {
			constraint = fmt.Sprintf("{\"advanced\":[{%s\"%s\":\"manual\", \"%s\":%f}]}", parameter.Precondition, parameter.ControlMode, parameter.Control, value)
		} else {
			constraint = fmt.Sprintf("{\"advanced\":[{%s\"%s\":%f}]}", parameter.Precondition, parameter.Control, value)
		}
		app.ApplyMediaTrackConstraints(ctx, constraint)
		settings, err := app.GetMediaTrackSettings(ctx)
		if err != nil {
			s.Fatal("Can't get settings: ", err)
		}
		if parameter.ControlMode != "" {
			getMode := *(*string)(unsafe.Pointer(reflect.ValueOf(&settings).Elem().FieldByName(strings.Title(parameter.ControlMode)).Pointer()))
			if getMode != "manual" {
				s.Error(parameter.ControlMode + " is not manual.")
			}
		}
		getValue := *(*float64)(unsafe.Pointer(reflect.ValueOf(&settings).Elem().FieldByName(strings.Title(parameter.Control)).Pointer()))
		differance := math.Abs(getValue - value)
		if differance > parameter.Tolerance {
			s.Errorf("Set %s to %f, get %f, differance %f, tolerance %f", parameter.Control, value, getValue, differance, parameter.Tolerance)
		}
	}
}
