// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"unsafe"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
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
	WhiteBalanceMode *string `json:"whiteBalanceMode,omitempty"`
	ExposureMode     *string `json:"exposureMode,omitempty"`
	FocusMode        *string `json:"focusMode,omitempty"`

	ExposureCompensation *float64 `json:"exposureCompensation,omitempty"`
	ExposureTime         *float64 `json:"exposureTime,omitempty"`
	ColorTemperature     *float64 `json:"colorTemperature,omitempty"`
	Iso                  *float64 `json:"iso,omitempty"`

	Brightness *float64 `json:"brightness,omitempty"`
	Contrast   *float64 `json:"contrast,omitempty"`
	Saturation *float64 `json:"saturation,omitempty"`
	Sharpness  *float64 `json:"sharpness,omitempty"`

	FocusDistance *float64 `json:"focusDistance,omitempty"`
	Pan           *float64 `json:"pan,omitempty"`
	Tilt          *float64 `json:"tilt,omitempty"`
	Zoom          *float64 `json:"zoom,omitempty"`
}

type mediaTrackConstraints struct {
	Advanced [1]mediaTrackSettings `json:"advanced"`
}

// From w3c standard MediaDeviceInfo type.
type mediaDeviceInfo struct {
	DeviceID string `json:"deviceId"`
	Label    string `json:"label"`
}

// Advanced control parameter.
type controlParameter struct {
	control     string
	isString    bool
	valueString string
	valueFloat  float64
}

// testParameter is used to verify different controls.
type testParameter struct {
	//   "control" The advanced control we want to verify.
	control string
	//   "prerequisite" The prerequisite for verifying the "control".
	prerequisite mediaTrackSettings
	//   "tolerance" The tolerance for get value and set value.
	tolerance float64
	//   "testValues" The values we set for "control".
	testValues []float64
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
func applyMediaTrackConstraints(ctx context.Context, conn *chrome.Conn, constraints mediaTrackConstraints) error {
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

	var cameras []mediaDeviceInfo
	if err := conn.Call(ctx, &cameras, "getCameras"); err != nil {
		s.Error("Failed to getCameras: ", err)
	}

	for _, camera := range cameras {
		s.Logf("Testing %s", camera.Label)
		if err := conn.Call(ctx, nil, "openPreview", camera.DeviceID); err != nil {
			s.Error("Failed to openPreview: ", err)
		}
		verifyAdvancedControls(ctx, s, conn)
		if err := conn.Call(ctx, nil, "closePreview"); err != nil {
			s.Error("Failed to closePreview: ", err)
		}
	}
}

func verifyAdvancedControls(ctx context.Context, s *testing.State, conn *chrome.Conn) {
	manual := "manual"
	continuous := "continuous"
	exposureTime333 := 333.0
	parameters := [...]testParameter{
		{"exposureTime", mediaTrackSettings{ExposureMode: &manual}, 1.0, []float64{0.0}},
		{"focusDistance", mediaTrackSettings{FocusMode: &manual}, 0.01, []float64{0.0}},
		{"colorTemperature", mediaTrackSettings{WhiteBalanceMode: &manual}, 0.0, []float64{0.0}},
		{"exposureCompensation", mediaTrackSettings{ExposureMode: &continuous}, 0.0, []float64{0.0}},
		{"iso", mediaTrackSettings{ExposureMode: &manual, ExposureTime: &exposureTime333}, 5.0, []float64{0.0}},
		{"brightness", mediaTrackSettings{}, 0.0, []float64{0.0}},
		{"contrast", mediaTrackSettings{}, 0.0, []float64{0.0}},
		{"saturation", mediaTrackSettings{}, 0.0, []float64{0.0}},
		{"sharpness", mediaTrackSettings{}, 0.0, []float64{0.0}},
		{"pan", mediaTrackSettings{}, 0.0, []float64{0.0}},
		{"tilt", mediaTrackSettings{}, 0.0, []float64{0.0}},
		{"zoom", mediaTrackSettings{}, 0.0, []float64{0.0}},
	}
	// We need to use specified values for color temperatures, because it is translated from white balance mode.
	availableColorTemperatures := []float64{2700, 3000, 5000, 5500, 6500, 7500, 15000}
	var supportedParameters []testParameter
	// We need separate test for the following controls. They need different exposure modes.
	var parameterExposureTime testParameter
	var parameterIso testParameter
	var parameterExposureCompensation testParameter
	capabilities, err := getMediaTrackCapabilities(ctx, conn)
	if err != nil {
		s.Fatal("Can't get capabilities: ", err)
		return
	}
	for _, parameter := range parameters {
		ss := reflect.ValueOf(&capabilities).Elem().FieldByName(strings.Title(parameter.control))
		if ss.IsNil() {
			continue
		}
		settingRange := *(*mediaSettingRange)(unsafe.Pointer(ss.Pointer()))
		s.Logf("support %s Range max:%f, min:%f, step:%f", parameter.control, settingRange.Max, settingRange.Min, settingRange.Step)
		// verify out of range values
		parameter.testValues = []float64{settingRange.Min - settingRange.Step, settingRange.Max + settingRange.Step}
		verifyControl(ctx, s, conn, parameter, false)

		totalSteps := (settingRange.Max - settingRange.Min) / settingRange.Step
		middleValue := settingRange.Min + settingRange.Step*math.Round(totalSteps/2)
		testValues := []float64{settingRange.Min, middleValue, settingRange.Max}
		if parameter.control == "colorTemperature" {
			testValues = []float64{}
			for _, value := range availableColorTemperatures {
				if value >= settingRange.Min && value <= settingRange.Max {
					testValues = append(testValues, value)
				}
			}
		}
		// only min values of focusDistances meaningful.
		if parameter.control == "focusDistance" {
			testValues = []float64{settingRange.Min, settingRange.Min + settingRange.Step}
		}

		// verify valid values
		parameter.testValues = testValues
		verifyControl(ctx, s, conn, parameter, true)

		// Store conflict parameters for later use.
		if parameter.control == "iso" {
			parameterIso = parameter
		} else if parameter.control == "exposureCompensation" {
			parameterExposureCompensation = parameter
		} else if parameter.control == "exposureTime" {
			parameterExposureTime = parameter
		} else {
			supportedParameters = append(supportedParameters, parameter)
		}
	}
	if parameterExposureTime.control != "" {
		testParameters := append(supportedParameters, parameterExposureTime)
		verifyAllControls(ctx, s, conn, testParameters)
	}
	if parameterIso.control != "" {
		testParameters := append(supportedParameters, parameterIso)
		verifyAllControls(ctx, s, conn, testParameters)
	}
	if parameterExposureCompensation.control != "" {
		testParameters := append(supportedParameters, parameterExposureCompensation)
		verifyAllControls(ctx, s, conn, testParameters)
	}
}

func setPrerequisite(settings, prerequisite *mediaTrackSettings) {
	v := reflect.ValueOf(prerequisite).Elem()
	for i := 0; i < v.NumField(); i++ {
		if !v.Field(i).IsNil() {
			fieldName := v.Type().Field(i).Name
			reflect.ValueOf(settings).Elem().FieldByName(fieldName).Set(v.Field(i))
		}
	}
}

func verifyControl(ctx context.Context, s *testing.State, conn *chrome.Conn, parameter testParameter, isValid bool) {
	for _, value := range parameter.testValues {
		var constraints mediaTrackConstraints
		settings, err := getMediaTrackSettings(ctx, conn)
		if err != nil {
			s.Fatal("Can't get settings: ", err)
		}
		origValue := *(*float64)(unsafe.Pointer(reflect.ValueOf(&settings).Elem().FieldByName(strings.Title(parameter.control)).Pointer()))
		constraints.Advanced[0] = parameter.prerequisite
		reflect.ValueOf(&constraints.Advanced[0]).Elem().FieldByName(strings.Title(parameter.control)).Set(reflect.ValueOf(&value))
		err = applyMediaTrackConstraints(ctx, conn, constraints)
		if isValid && err != nil {
			s.Fatal("Can't apply constraints: ", err)
		}
		settings, err = getMediaTrackSettings(ctx, conn)
		if err != nil {
			s.Fatal("Can't get settings: ", err)
		}
		getValue := *(*float64)(unsafe.Pointer(reflect.ValueOf(&settings).Elem().FieldByName(strings.Title(parameter.control)).Pointer()))
		differance := math.Abs(getValue - value)
		if isValid && differance > parameter.tolerance {
			s.Errorf("Set %s to %f, get %f, differance %f, tolerance %f", parameter.control, value, getValue, differance, parameter.tolerance)
		}
		if !isValid && getValue != origValue {
			s.Errorf("%s: Get value %f != Original value %f", parameter.control, getValue, origValue)
		}
	}
}

func verifyAllControls(ctx context.Context, s *testing.State, conn *chrome.Conn, parameters []testParameter) {
	var constraints mediaTrackConstraints
	for _, parameter := range parameters {
		setPrerequisite(&constraints.Advanced[0], &parameter.prerequisite)
		reflect.ValueOf(&constraints.Advanced[0]).Elem().FieldByName(strings.Title(parameter.control)).Set(reflect.ValueOf(&parameter.testValues[0]))
	}
	err := applyMediaTrackConstraints(ctx, conn, constraints)
	if err != nil {
		s.Fatal("Can't apply constraints: ", err)
	}

	settings, err := getMediaTrackSettings(ctx, conn)
	if err != nil {
		s.Fatal("Can't get settings: ", err)
	}
	for _, parameter := range parameters {
		getValue := *(*float64)(unsafe.Pointer(reflect.ValueOf(&settings).Elem().FieldByName(strings.Title(parameter.control)).Pointer()))
		differance := math.Abs(getValue - parameter.testValues[0])
		if differance > parameter.tolerance {
			s.Errorf("Set %s to %f, get %f, differance %f, tolerance %f", parameter.control, parameter.testValues[0], getValue, differance, parameter.tolerance)
		}
	}
}
