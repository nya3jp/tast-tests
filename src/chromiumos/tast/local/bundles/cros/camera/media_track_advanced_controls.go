// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"

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
	ExposureMode     *[]string `json:"exposureMode"`
	FocusMode        *[]string `json:"focusMode"`
	WhiteBalanceMode *[]string `json:"whiteBalanceMode"`

	Brightness           *mediaSettingRange `json:"brightness"`
	ColorTemperature     *mediaSettingRange `json:"colorTemperature"`
	Contrast             *mediaSettingRange `json:"contrast"`
	ExposureCompensation *mediaSettingRange `json:"exposureCompensation"`
	ExposureTime         *mediaSettingRange `json:"exposureTime"`
	FocusDistance        *mediaSettingRange `json:"focusDistance"`
	Iso                  *mediaSettingRange `json:"iso"`
	Pan                  *mediaSettingRange `json:"pan"`
	Saturation           *mediaSettingRange `json:"saturation"`
	Sharpness            *mediaSettingRange `json:"sharpness"`
	Tilt                 *mediaSettingRange `json:"tilt"`
	Zoom                 *mediaSettingRange `json:"zoom"`
}

// mediaTrackSettings is used to return the current values configured for each of a MediaStreamTrack's settings.
type mediaTrackSettings struct {
	ExposureMode     *string `json:"exposureMode,omitempty"`
	FocusMode        *string `json:"focusMode,omitempty"`
	WhiteBalanceMode *string `json:"whiteBalanceMode,omitempty"`

	Brightness           *float64 `json:"brightness,omitempty"`
	ColorTemperature     *float64 `json:"colorTemperature,omitempty"`
	Contrast             *float64 `json:"contrast,omitempty"`
	ExposureCompensation *float64 `json:"exposureCompensation,omitempty"`
	ExposureTime         *float64 `json:"exposureTime,omitempty"`
	FocusDistance        *float64 `json:"focusDistance,omitempty"`
	Iso                  *float64 `json:"iso,omitempty"`
	Pan                  *float64 `json:"pan,omitempty"`
	Saturation           *float64 `json:"saturation,omitempty"`
	Sharpness            *float64 `json:"sharpness,omitempty"`
	Tilt                 *float64 `json:"tilt,omitempty"`
	Zoom                 *float64 `json:"zoom,omitempty"`
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
	// control is the advanced control we want to verify.
	control mediaControl
	// prerequisite is the prerequisite for verifying the "control".
	prerequisite mediaTrackSettings
	// tolerance is the tolerance for get value and set value.
	tolerance float64
	// testValues is the values we set for "control".
	testValues []float64
}

type mediaControl interface {
	getName() string
	getSettingRange(*mediaTrackCapabilities) *mediaSettingRange
	getSettingFloatField(*mediaTrackSettings) **float64
	getSettingStringField(*mediaTrackSettings) **string
}

type brightnessControl struct {
	mediaControl
}

func (c *brightnessControl) getName() string {
	return "brightness"
}

func (c *brightnessControl) getSettingRange(capabilities *mediaTrackCapabilities) *mediaSettingRange {
	return (*capabilities).Brightness
}

func (c *brightnessControl) getSettingFloatField(settings *mediaTrackSettings) **float64 {
	return &(*settings).Brightness
}

type colorTemperatureControl struct {
	mediaControl
}

func (c *colorTemperatureControl) getName() string {
	return "colorTemperature"
}

func (c *colorTemperatureControl) getSettingRange(capabilities *mediaTrackCapabilities) *mediaSettingRange {
	return (*capabilities).ColorTemperature
}

func (c *colorTemperatureControl) getSettingFloatField(settings *mediaTrackSettings) **float64 {
	return &(*settings).ColorTemperature
}

type contrastControl struct {
	mediaControl
}

func (c *contrastControl) getName() string {
	return "contrast"
}

func (c *contrastControl) getSettingRange(capabilities *mediaTrackCapabilities) *mediaSettingRange {
	return (*capabilities).Contrast
}

func (c *contrastControl) getSettingFloatField(settings *mediaTrackSettings) **float64 {
	return &(*settings).Contrast
}

type exposureCompensationControl struct {
	mediaControl
}

func (c *exposureCompensationControl) getName() string {
	return "exposureCompensation"
}

func (c *exposureCompensationControl) getSettingRange(capabilities *mediaTrackCapabilities) *mediaSettingRange {
	return (*capabilities).ExposureCompensation
}

func (c *exposureCompensationControl) getSettingFloatField(settings *mediaTrackSettings) **float64 {
	return &(*settings).ExposureCompensation
}

type exposureTimeControl struct {
	mediaControl
}

func (c *exposureTimeControl) getName() string {
	return "exposureTime"
}

func (c *exposureTimeControl) getSettingFloatField(settings *mediaTrackSettings) **float64 {
	return &(*settings).ExposureTime
}

func (c *exposureTimeControl) getSettingRange(capabilities *mediaTrackCapabilities) *mediaSettingRange {
	return (*capabilities).ExposureTime
}

type focusDistanceControl struct {
	mediaControl
}

func (c *focusDistanceControl) getName() string {
	return "focusDistance"
}

func (c *focusDistanceControl) getSettingRange(capabilities *mediaTrackCapabilities) *mediaSettingRange {
	return (*capabilities).FocusDistance
}

func (c *focusDistanceControl) getSettingFloatField(settings *mediaTrackSettings) **float64 {
	return &(*settings).FocusDistance
}

type isoControl struct {
	mediaControl
}

func (c *isoControl) getName() string {
	return "iso"
}

func (c *isoControl) getSettingRange(capabilities *mediaTrackCapabilities) *mediaSettingRange {
	return (*capabilities).Iso
}

func (c *isoControl) getSettingFloatField(settings *mediaTrackSettings) **float64 {
	return &(*settings).Iso
}

type panControl struct {
	mediaControl
}

func (c *panControl) getName() string {
	return "pan"
}

func (c *panControl) getSettingRange(capabilities *mediaTrackCapabilities) *mediaSettingRange {
	return (*capabilities).Pan
}

func (c *panControl) getSettingFloatField(settings *mediaTrackSettings) **float64 {
	return &(*settings).Pan
}

type saturationControl struct {
	mediaControl
}

func (c *saturationControl) getName() string {
	return "saturation"
}

func (c *saturationControl) getSettingRange(capabilities *mediaTrackCapabilities) *mediaSettingRange {
	return (*capabilities).Saturation
}

func (c *saturationControl) getSettingFloatField(settings *mediaTrackSettings) **float64 {
	return &(*settings).Saturation
}

type sharpnessControl struct {
	mediaControl
}

func (c *sharpnessControl) getName() string {
	return "sharpness"
}

func (c *sharpnessControl) getSettingRange(capabilities *mediaTrackCapabilities) *mediaSettingRange {
	return (*capabilities).Sharpness
}

func (c *sharpnessControl) getSettingFloatField(settings *mediaTrackSettings) **float64 {
	return &(*settings).Sharpness
}

type tiltControl struct {
	mediaControl
}

func (c *tiltControl) getName() string {
	return "tilt"
}

func (c *tiltControl) getSettingRange(capabilities *mediaTrackCapabilities) *mediaSettingRange {
	return (*capabilities).Tilt
}

func (c *tiltControl) getSettingFloatField(settings *mediaTrackSettings) **float64 {
	return &(*settings).Tilt
}

type zoomControl struct {
	mediaControl
}

func (c *zoomControl) getName() string {
	return "zoom"
}

func (c *zoomControl) getSettingRange(capabilities *mediaTrackCapabilities) *mediaSettingRange {
	return (*capabilities).Zoom
}

func (c *zoomControl) getSettingFloatField(settings *mediaTrackSettings) **float64 {
	return &(*settings).Zoom
}

type exposureModeControl struct {
	mediaControl
}

func (c *exposureModeControl) getName() string {
	return "exposureMode"
}

func (c *exposureModeControl) getSettingStringField(settings *mediaTrackSettings) **string {
	return &(*settings).ExposureMode
}

type focusModeControl struct {
	mediaControl
}

func (c *focusModeControl) getName() string {
	return "focusMode"
}

func (c *focusModeControl) getSettingStringField(settings *mediaTrackSettings) **string {
	return &(*settings).FocusMode
}

type whiteBalanceModeControl struct {
	mediaControl
}

func (c *whiteBalanceModeControl) getName() string {
	return "whiteBalanceMode"
}

func (c *whiteBalanceModeControl) getSettingStringField(settings *mediaTrackSettings) **string {
	return &(*settings).WhiteBalanceMode
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
		jsondata, _ := json.Marshal(constraints)
		return errors.Wrapf(err, "failed to apply constraints %s", string(jsondata))
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
		s.Log("Testing ", camera.Label)
		if err := conn.Call(ctx, nil, "openPreview", camera.DeviceID); err != nil {
			s.Fatal("Failed to openPreview: ", err)
		}
		verifyAdvancedControls(ctx, s, conn)
		if err := conn.Call(ctx, nil, "closePreview"); err != nil {
			s.Fatal("Failed to closePreview: ", err)
		}
	}
}

func verifyAdvancedControls(ctx context.Context, s *testing.State, conn *chrome.Conn) {
	manual := "manual"
	continuous := "continuous"
	exposureTime333 := 333.0
	parameters := [...]testParameter{
		{&exposureTimeControl{}, mediaTrackSettings{ExposureMode: &manual}, 1.0, []float64{0.0}},
		{&focusDistanceControl{}, mediaTrackSettings{FocusMode: &manual}, 0.01, []float64{0.0}},
		{&colorTemperatureControl{}, mediaTrackSettings{WhiteBalanceMode: &manual}, 0.0, []float64{0.0}},
		{&exposureCompensationControl{}, mediaTrackSettings{ExposureMode: &continuous}, 0.0, []float64{0.0}},
		{&isoControl{}, mediaTrackSettings{ExposureMode: &manual, ExposureTime: &exposureTime333}, 5.0, []float64{0.0}},
		{&brightnessControl{}, mediaTrackSettings{}, 0.0, []float64{0.0}},
		{&contrastControl{}, mediaTrackSettings{}, 0.0, []float64{0.0}},
		{&saturationControl{}, mediaTrackSettings{}, 0.0, []float64{0.0}},
		{&sharpnessControl{}, mediaTrackSettings{}, 0.0, []float64{0.0}},
		{&panControl{}, mediaTrackSettings{}, 0.0, []float64{0.0}},
		{&tiltControl{}, mediaTrackSettings{}, 0.0, []float64{0.0}},
		{&zoomControl{}, mediaTrackSettings{}, 0.0, []float64{0.0}},
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
		settingRangeAddr := parameter.control.getSettingRange(&capabilities)
		if settingRangeAddr == nil {
			continue
		}
		settingRange := *settingRangeAddr
		s.Logf("support %s Range max:%f, min:%f, step:%f", parameter.control.getName(), settingRange.Max, settingRange.Min, settingRange.Step)
		// verify out of range values
		parameter.testValues = []float64{settingRange.Min - settingRange.Step, settingRange.Max + settingRange.Step}
		verifyControl(ctx, s, conn, parameter, false)

		totalSteps := (settingRange.Max - settingRange.Min) / settingRange.Step
		middleValue := settingRange.Min + settingRange.Step*math.Round(totalSteps/2)
		testValues := []float64{settingRange.Min, middleValue, settingRange.Max}
		if parameter.control.getName() == "colorTemperature" {
			testValues = []float64{}
			for _, value := range availableColorTemperatures {
				if value >= settingRange.Min && value <= settingRange.Max {
					testValues = append(testValues, value)
				}
			}
		}
		// only min values of focusDistances meaningful.
		if parameter.control.getName() == "focusDistance" {
			testValues = []float64{settingRange.Min, settingRange.Min + settingRange.Step}
		}

		// verify valid values
		parameter.testValues = testValues
		verifyControl(ctx, s, conn, parameter, true)

		// Store conflict parameters for later use.
		if parameter.control.getName() == "iso" {
			parameterIso = parameter
		} else if parameter.control.getName() == "exposureCompensation" {
			parameterExposureCompensation = parameter
		} else if parameter.control.getName() == "exposureTime" {
			parameterExposureTime = parameter
		} else {
			supportedParameters = append(supportedParameters, parameter)
		}
	}
	if parameterExposureTime.control != nil {
		s.Log("verify all controls with ", parameterExposureTime.control.getName())
		testParameters := append(supportedParameters, parameterExposureTime)
		verifyAllControls(ctx, s, conn, testParameters)
	}
	if parameterIso.control != nil {
		s.Log("verify all controls with ", parameterIso.control.getName())
		testParameters := append(supportedParameters, parameterIso)
		verifyAllControls(ctx, s, conn, testParameters)
	}
	if parameterExposureCompensation.control != nil {
		s.Log("verify all controls with ", parameterExposureCompensation.control.getName())
		testParameters := append(supportedParameters, parameterExposureCompensation)
		verifyAllControls(ctx, s, conn, testParameters)
	}
}

func setPrerequisite(settings, prerequisite *mediaTrackSettings) {
	allStringFields := [...]mediaControl{&exposureModeControl{}, &focusModeControl{}, &whiteBalanceModeControl{}}
	allFloatFields := [...]mediaControl{
		&colorTemperatureControl{},
		&contrastControl{},
		&exposureCompensationControl{},
		&exposureTimeControl{},
		&focusDistanceControl{},
		&isoControl{},
		&panControl{},
		&saturationControl{},
		&sharpnessControl{},
		&tiltControl{},
		&zoomControl{}}
	for _, field := range allStringFields {
		valueAddr := field.getSettingStringField(prerequisite)
		if valueAddr != nil && *valueAddr != nil {
			*field.getSettingStringField(settings) = *valueAddr
		}
	}
	for _, field := range allFloatFields {
		valueAddr := field.getSettingFloatField(prerequisite)
		if valueAddr != nil && *valueAddr != nil {
			*field.getSettingFloatField(settings) = *valueAddr
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
		origValue := **parameter.control.getSettingFloatField(&settings)
		setPrerequisite(&constraints.Advanced[0], &parameter.prerequisite)
		*parameter.control.getSettingFloatField(&constraints.Advanced[0]) = &value
		err = applyMediaTrackConstraints(ctx, conn, constraints)
		if isValid && err != nil {
			s.Fatal("Can't apply constraints: ", err)
		}
		settings, err = getMediaTrackSettings(ctx, conn)
		if err != nil {
			s.Fatal("Can't get settings: ", err)
		}
		getValue := **parameter.control.getSettingFloatField(&settings)
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
		*parameter.control.getSettingFloatField(&constraints.Advanced[0]) = &parameter.testValues[0]
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
		getValue := **parameter.control.getSettingFloatField(&settings)
		differance := math.Abs(getValue - parameter.testValues[0])
		if differance > parameter.tolerance {
			s.Errorf("Set %s to %f, get %f, differance %f, tolerance %f", parameter.control, parameter.testValues[0], getValue, differance, parameter.tolerance)
		}
	}
}
