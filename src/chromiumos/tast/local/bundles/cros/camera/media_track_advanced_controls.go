// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"unsafe"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/pre"
	"chromiumos/tast/testing"
)

// mediaSettingRange provides the possible range and value.
type mediaSettingRange struct {
	Max  float64 `json:"max"`
	Min  float64 `json:"min"`
	Step float64 `json:"step"`
}

// mediaTrackCapabilities specifies the values or range of values of each constrainable property.
type mediaTrackCapabilities struct {
	WhiteBalanceMode *[]string `json:"whiteBalanceMode"`
	ExposureMode     *[]string `json:"exposureMode"`
	FocusMode        *[]string `json:"focusMode"`

	ExposureCompensation *mediaSettingRange `json:"exposureCompensation"`
	ExposureTime         *mediaSettingRange `json:"exposureTime"`
	ColorTemperature     *mediaSettingRange `json:"colorTemperature"`
	Iso                  *mediaSettingRange `json:"iso"`

	Brightness *mediaSettingRange `json:"brightness"`
	Contrast   *mediaSettingRange `json:"contrast"`
	Saturation *mediaSettingRange `json:"saturation"`
	Sharpness  *mediaSettingRange `json:"sharpness"`

	FocusDistance *mediaSettingRange `json:"focusDistance"`
	Pan           *mediaSettingRange `json:"pan"`
	Tilt          *mediaSettingRange `json:"tilt"`
	Zoom          *mediaSettingRange `json:"zoom"`
}

// mediaTrackSettings is used to return the current values configured for each of a MediaStreamTrack's settings.
type mediaTrackSettings struct {
	WhiteBalanceMode *string `json:"whiteBalanceMode"`
	ExposureMode     *string `json:"exposureMode"`
	FocusMode        *string `json:"focusMode"`

	ExposureCompensation *float64 `json:"exposureCompensation"`
	ExposureTime         *float64 `json:"exposureTime"`
	ColorTemperature     *float64 `json:"colorTemperature"`
	Iso                  *float64 `json:"iso"`

	Brightness *float64 `json:"brightness"`
	Contrast   *float64 `json:"contrast"`
	Saturation *float64 `json:"saturation"`
	Sharpness  *float64 `json:"sharpness"`

	FocusDistance *float64 `json:"focusDistance"`
	Pan           *float64 `json:"pan"`
	Tilt          *float64 `json:"tilt"`
	Zoom          *float64 `json:"zoom"`
}

type testParameter struct {
	Control string
	// The related control for |Control|.
	// e.g. If |Control| is "exposureTime", |ControlMode| is "exposureMode".
	ControlMode  string
	Precondition string
	Tolerance    float64
	TestValues   []float64
}

// getMediaTrackCapabilities gets |MediaTrackCapabilities| from preview video track.
func getMediaTrackCapabilities(ctx context.Context, conn *chrome.Conn) (mediaTrackCapabilities, error) {
	var capabilities mediaTrackCapabilities
	if err := conn.Call(ctx, &capabilities, "getMediaTrackCapabilities"); err != nil {
		return capabilities, errors.Wrap(err, "failed to get MediaTrackCapabilities")
	}
	return capabilities, nil
}

// getMediaTrackSettings gets |MediaTrackSettings| from preview video track.
func getMediaTrackSettings(ctx context.Context, conn *chrome.Conn) (mediaTrackSettings, error) {
	var settings mediaTrackSettings
	if err := conn.Call(ctx, &settings, "getMediaTrackSettings"); err != nil {
		return settings, errors.Wrap(err, "failed to get MediaTrackSettings")
	}
	return settings, nil
}

// applyMediaTrackConstraints applies |constraints| to preview video track.
func applyMediaTrackConstraints(ctx context.Context, conn *chrome.Conn, constraints string) error {
	if err := conn.Call(ctx, nil, "applyMediaTrackConstraints", constraints); err != nil {
		return errors.Wrapf(err, "failed to apply constraints %s", constraints)
	}
	return nil
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         MediaTrackAdvancedControls,
		Desc:         "Verifies the MediaTrack advanced controls",
		Contacts:     []string{"mojahsu@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", caps.BuiltinOrVividCamera},
		Data:         []string{"media_track_advanced_controls.html", "media_track_advanced_controls.js"},
		Pre:          pre.ChromeVideo(),
	})
}

func MediaTrackAdvancedControls(ctx context.Context, s *testing.State) {
	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	cr := s.PreValue().(*chrome.Chrome)
	conn, err := cr.NewConn(ctx, server.URL+"/media_track_advanced_controls.html")
	if err != nil {
		s.Fatal("Failed to open testing page: ", err)
	}
	defer conn.Close()

	var numCameras int
	if err := conn.Call(ctx, &numCameras, "getNumOfCameras"); err != nil {
		s.Error("Failed to getNumOfCameras: ", err)
	}
	s.Logf("support %d cameras", numCameras)

	for i := 0; i < numCameras; i++ {
		var label string
		if err := conn.Call(ctx, &label, "getCameraLabel", i); err != nil {
			s.Error("Failed to getCameraLabel: ", err)
		}
		s.Logf("Testing %s", label)
		if err := conn.Call(ctx, nil, "playVideo", i); err != nil {
			s.Error("Failed to playVideo: ", err)
		}
		verifyAdvancedControls(ctx, s, conn)
		if err := conn.Call(ctx, nil, "stopVideo"); err != nil {
			s.Error("Failed to stopVideo: ", err)
		}
	}
}

func verifyAdvancedControls(ctx context.Context, s *testing.State, conn *chrome.Conn) {
	parameters := [12]testParameter{
		{"exposureTime", "exposureMode", "", 1.0, []float64{0.0}},
		{"focusDistance", "focusMode", "", 0.01, []float64{0.0}},
		{"colorTemperature", "whiteBalanceMode", "", 0.0, []float64{0.0}},
		{"exposureCompensation", "", `"exposureMode":"continuous",`, 0.0, []float64{0.0}},
		{"iso", "", `"exposureMode":"manual", "exposureTime":333,`, 5.0, []float64{0.0}},
		{"brightness", "", "", 0.0, []float64{0.0}},
		{"contrast", "", "", 0.0, []float64{0.0}},
		{"saturation", "", "", 0.0, []float64{0.0}},
		{"sharpness", "", "", 0.0, []float64{0.0}},
		{"pan", "", "", 0.0, []float64{0.0}},
		{"tilt", "", "", 0.0, []float64{0.0}},
		{"zoom", "", "", 0.0, []float64{0.0}},
	}
	// We need to use specified values for color temperatures, because it is translated from white balance mode.
	availableColorTemperatures := []float64{2700, 3000, 5000, 5500, 6500, 7500, 15000}
	var supportedParameters []testParameter
	// We need separate test for the following controls. They need different exposure modes.
	parameterExposureTime := testParameter{"", "", "", 0.0, []float64{0.0}}
	parameterIso := testParameter{"", "", "", 0.0, []float64{0.0}}
	parameterExposureCompensation := testParameter{"", "", "", 0.0, []float64{0.0}}
	capabilities, err := getMediaTrackCapabilities(ctx, conn)
	if err != nil {
		s.Fatal("Can't get capabilities: ", err)
		return
	}
	for _, parameter := range parameters {
		ss := reflect.ValueOf(&capabilities).Elem().FieldByName(strings.Title(parameter.Control))
		if ss.IsNil() {
			continue
		}
		settingRange := *(*mediaSettingRange)(unsafe.Pointer(ss.Pointer()))
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

		// verify out of range values
		parameter.TestValues = []float64{settingRange.Min - settingRange.Step, settingRange.Max + settingRange.Step}
		verifyControl(ctx, s, conn, parameter, false)

		totalSteps := (settingRange.Max - settingRange.Min) / settingRange.Step
		middleValue := settingRange.Min + settingRange.Step*math.Round(totalSteps/2)
		testValues := []float64{settingRange.Min, middleValue, settingRange.Max}
		if parameter.Control == "colorTemperature" {
			testValues = []float64{}
			for _, value := range availableColorTemperatures {
				if value >= settingRange.Min && value <= settingRange.Max {
					testValues = append(testValues, value)
				}
			}
		}

		// verify valid values
		parameter.TestValues = testValues
		verifyControl(ctx, s, conn, parameter, true)

		// Store conflict parameters for later use.
		if parameter.Control == "iso" {
			parameterIso = parameter
		} else if parameter.Control == "exposureCompensation" {
			parameterExposureCompensation = parameter
		} else if parameter.Control == "exposureTime" {
			parameterExposureTime = parameter
		} else {
			supportedParameters = append(supportedParameters, parameter)
		}
	}
	if parameterExposureTime.Control != "" {
		testParameters := append(supportedParameters, parameterExposureTime)
		verifyAllControls(ctx, s, conn, testParameters)
	}
	if parameterIso.Control != "" {
		testParameters := append(supportedParameters, parameterIso)
		verifyAllControls(ctx, s, conn, testParameters)
	}
	if parameterExposureCompensation.Control != "" {
		testParameters := append(supportedParameters, parameterExposureCompensation)
		verifyAllControls(ctx, s, conn, testParameters)
	}
}

func verifyControl(ctx context.Context, s *testing.State, conn *chrome.Conn, parameter testParameter, isValid bool) {
	for _, value := range parameter.TestValues {
		var constraint string
		var origMode string
		settings, err := getMediaTrackSettings(ctx, conn)
		origValue := *(*float64)(unsafe.Pointer(reflect.ValueOf(&settings).Elem().FieldByName(strings.Title(parameter.Control)).Pointer()))
		if parameter.ControlMode != "" {
			origMode = *(*string)(unsafe.Pointer(reflect.ValueOf(&settings).Elem().FieldByName(strings.Title(parameter.ControlMode)).Pointer()))
			constraint = fmt.Sprintf(`{"advanced":[{%s"%s":"manual", "%s":%f}]}`, parameter.Precondition, parameter.ControlMode, parameter.Control, value)
		} else {
			constraint = fmt.Sprintf(`{"advanced":[{%s"%s":%f}]}`, parameter.Precondition, parameter.Control, value)
		}
		applyMediaTrackConstraints(ctx, conn, constraint)
		settings, err = getMediaTrackSettings(ctx, conn)
		if err != nil {
			s.Fatal("Can't get settings: ", err)
		}
		if parameter.ControlMode != "" {
			getMode := *(*string)(unsafe.Pointer(reflect.ValueOf(&settings).Elem().FieldByName(strings.Title(parameter.ControlMode)).Pointer()))
			if isValid && getMode != "manual" {
				s.Error(parameter.ControlMode + " is not manual.")
			}
			if !isValid && getMode != origMode {
				s.Errorf("%s: Get mode %s != Original mode %s", parameter.Control, getMode, origMode)
			}
		}
		getValue := *(*float64)(unsafe.Pointer(reflect.ValueOf(&settings).Elem().FieldByName(strings.Title(parameter.Control)).Pointer()))
		differance := math.Abs(getValue - value)
		if isValid && differance > parameter.Tolerance {
			s.Errorf("Set %s to %f, get %f, differance %f, tolerance %f", parameter.Control, value, getValue, differance, parameter.Tolerance)
		}
		if !isValid && getValue != origValue {
			s.Errorf("%s: Get value %f != Original value %f", parameter.Control, getValue, origValue)
		}
	}
}

func verifyAllControls(ctx context.Context, s *testing.State, conn *chrome.Conn, parameters []testParameter) {
	constraint := `{"advanced":[{`
	for _, parameter := range parameters {
		if parameter.ControlMode != "" {
			constraint += fmt.Sprintf(`%s"%s":"manual", "%s":%f, `, parameter.Precondition, parameter.ControlMode, parameter.Control, parameter.TestValues[0])
		} else {
			constraint += fmt.Sprintf(`%s"%s":%f, `, parameter.Precondition, parameter.Control, parameter.TestValues[0])
		}
	}
	// remove the last ", "
	constraint = constraint[:len(constraint)-2]
	constraint += "}]}"
	applyMediaTrackConstraints(ctx, conn, constraint)

	settings, err := getMediaTrackSettings(ctx, conn)
	if err != nil {
		s.Fatal("Can't get settings: ", err)
	}
	for _, parameter := range parameters {
		if parameter.ControlMode != "" {
			getMode := *(*string)(unsafe.Pointer(reflect.ValueOf(&settings).Elem().FieldByName(strings.Title(parameter.ControlMode)).Pointer()))
			if getMode != "manual" {
				s.Error(parameter.ControlMode + " is not manual.")
			}
		}
		getValue := *(*float64)(unsafe.Pointer(reflect.ValueOf(&settings).Elem().FieldByName(strings.Title(parameter.Control)).Pointer()))
		differance := math.Abs(getValue - parameter.TestValues[0])
		if differance > parameter.Tolerance {
			s.Errorf("Set %s to %f, get %f, differance %f, tolerance %f", parameter.Control, parameter.TestValues[0], getValue, differance, parameter.Tolerance)
		}
	}
}
