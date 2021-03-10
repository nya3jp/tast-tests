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
// from https://w3c.github.io/mediacapture-main/#media-track-capabilities.
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
// from https://w3c.github.io/mediacapture-main/#media-track-settings.
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
// from https://w3c.github.io/mediacapture-main/#ref-for-dom-mediadeviceinfo-11.
type mediaDeviceInfo struct {
	DeviceID string `json:"deviceId"`
	Label    string `json:"label"`
}

type mediaControl interface {
	getName() string
	// return nil if the platform doesn't support such control
	getSettingRange(*mediaTrackCapabilities) *mediaSettingRange
	getValue(*mediaTrackSettings, *float64)
	setValue(*mediaTrackSettings, *float64)
	getTolerance() float64
	isEqual(float64, float64) bool
	initValidTestValues(mediaSettingRange)
	initInvalidTestValues(mediaSettingRange)
	getValidTestValues() []float64
	getInvalidTestValues() []float64
	// prerequisite is the prerequisite for the control.
	getPrerequisite() mediaTrackSettings
}

type defaultControl struct {
	validTestValues   []float64
	invalidTestValues []float64
	mediaControl
}

func (c *defaultControl) getTolerance() float64 {
	return 0.0
}

func (c *defaultControl) isEqual(v1, v2 float64) bool {
	differance := math.Abs(v1 - v2)
	return differance <= c.getTolerance()
}

func (c *defaultControl) initValidTestValues(r mediaSettingRange) {
	totalSteps := (r.Max - r.Min) / r.Step
	middleValue := r.Min + r.Step*math.Round(totalSteps/2)
	(*c).validTestValues = []float64{r.Min, middleValue, r.Max}
}

func (c *defaultControl) initInvalidTestValues(r mediaSettingRange) {
	(*c).invalidTestValues = []float64{r.Min - r.Step, r.Max + r.Step}
}

func (c *defaultControl) getValidTestValues() []float64 {
	return (*c).validTestValues
}

func (c *defaultControl) getInvalidTestValues() []float64 {
	return (*c).invalidTestValues
}

func (c *defaultControl) getPrerequisite() mediaTrackSettings {
	return mediaTrackSettings{}
}

type brightnessControl struct {
	defaultControl
}

func (c *brightnessControl) getName() string {
	return "brightness"
}

func (c *brightnessControl) getSettingRange(capabilities *mediaTrackCapabilities) *mediaSettingRange {
	return (*capabilities).Brightness
}

func (c *brightnessControl) getValue(settings *mediaTrackSettings, value *float64) {
	*value = *(*settings).Brightness
}

func (c *brightnessControl) setValue(settings *mediaTrackSettings, value *float64) {
	(*settings).Brightness = value
}

type colorTemperatureControl struct {
	defaultControl
}

func (c *colorTemperatureControl) getName() string {
	return "colorTemperature"
}

func (c *colorTemperatureControl) getSettingRange(capabilities *mediaTrackCapabilities) *mediaSettingRange {
	return (*capabilities).ColorTemperature
}

func (c *colorTemperatureControl) getValue(settings *mediaTrackSettings, value *float64) {
	*value = *(*settings).ColorTemperature
}

func (c *colorTemperatureControl) setValue(settings *mediaTrackSettings, value *float64) {
	(*settings).ColorTemperature = value
}

func (c *colorTemperatureControl) initValidTestValues(r mediaSettingRange) {
	// We need to use specified values for color temperatures, because it is translated from white balance mode.
	availableColorTemperatures := []float64{2700, 3000, 5000, 5500, 6500, 7500, 15000}
	c.validTestValues = []float64{}
	for _, value := range availableColorTemperatures {
		if value >= r.Min && value <= r.Max {
			c.validTestValues = append(c.validTestValues, value)
		}
	}
}

func (c *colorTemperatureControl) getPrerequisite() mediaTrackSettings {
	manual := "manual"
	return mediaTrackSettings{WhiteBalanceMode: &manual}
}

type contrastControl struct {
	defaultControl
}

func (c *contrastControl) getName() string {
	return "contrast"
}

func (c *contrastControl) getSettingRange(capabilities *mediaTrackCapabilities) *mediaSettingRange {
	return (*capabilities).Contrast
}

func (c *contrastControl) getValue(settings *mediaTrackSettings, value *float64) {
	*value = *(*settings).Contrast
}

func (c *contrastControl) setValue(settings *mediaTrackSettings, value *float64) {
	(*settings).Contrast = value
}

type exposureCompensationControl struct {
	defaultControl
}

func (c *exposureCompensationControl) getName() string {
	return "exposureCompensation"
}

func (c *exposureCompensationControl) getSettingRange(capabilities *mediaTrackCapabilities) *mediaSettingRange {
	return (*capabilities).ExposureCompensation
}

func (c *exposureCompensationControl) getValue(settings *mediaTrackSettings, value *float64) {
	*value = *(*settings).ExposureCompensation
}

func (c *exposureCompensationControl) setValue(settings *mediaTrackSettings, value *float64) {
	(*settings).ExposureCompensation = value
}

func (c *exposureCompensationControl) getPrerequisite() mediaTrackSettings {
	continuous := "continuous"
	return mediaTrackSettings{ExposureMode: &continuous}
}

type exposureTimeControl struct {
	defaultControl
}

func (c *exposureTimeControl) getName() string {
	return "exposureTime"
}

func (c *exposureTimeControl) getValue(settings *mediaTrackSettings, value *float64) {
	*value = *(*settings).ExposureTime
}

func (c *exposureTimeControl) setValue(settings *mediaTrackSettings, value *float64) {
	(*settings).ExposureTime = value
}

func (c *exposureTimeControl) getSettingRange(capabilities *mediaTrackCapabilities) *mediaSettingRange {
	return (*capabilities).ExposureTime
}

func (c *exposureTimeControl) getTolerance() float64 {
	return 1.0
}

func (c *exposureTimeControl) getPrerequisite() mediaTrackSettings {
	manual := "manual"
	return mediaTrackSettings{ExposureMode: &manual}
}

type focusDistanceControl struct {
	defaultControl
}

func (c *focusDistanceControl) getName() string {
	return "focusDistance"
}

func (c *focusDistanceControl) getSettingRange(capabilities *mediaTrackCapabilities) *mediaSettingRange {
	return (*capabilities).FocusDistance
}

func (c *focusDistanceControl) getValue(settings *mediaTrackSettings, value *float64) {
	*value = *(*settings).FocusDistance
}

func (c *focusDistanceControl) setValue(settings *mediaTrackSettings, value *float64) {
	(*settings).FocusDistance = value
}

func (c *focusDistanceControl) getTolerance() float64 {
	return 0.01
}

func (c *focusDistanceControl) initValidTestValues(r mediaSettingRange) {
	// only min values of focusDistances meaningful.
	(*c).validTestValues = []float64{r.Min, r.Min + r.Step}
}

func (c *focusDistanceControl) getPrerequisite() mediaTrackSettings {
	manual := "manual"
	return mediaTrackSettings{FocusMode: &manual}
}

type isoControl struct {
	defaultControl
}

func (c *isoControl) getName() string {
	return "iso"
}

func (c *isoControl) getSettingRange(capabilities *mediaTrackCapabilities) *mediaSettingRange {
	return (*capabilities).Iso
}

func (c *isoControl) getValue(settings *mediaTrackSettings, value *float64) {
	*value = *(*settings).Iso
}

func (c *isoControl) setValue(settings *mediaTrackSettings, value *float64) {
	(*settings).Iso = value
}

func (c *isoControl) getTolerance() float64 {
	return 5.0
}

func (c *isoControl) getPrerequisite() mediaTrackSettings {
	manual := "manual"
	exposureTime333 := 333.0
	return mediaTrackSettings{ExposureMode: &manual, ExposureTime: &exposureTime333}
}

type panControl struct {
	defaultControl
}

func (c *panControl) getName() string {
	return "pan"
}

func (c *panControl) getSettingRange(capabilities *mediaTrackCapabilities) *mediaSettingRange {
	return (*capabilities).Pan
}

func (c *panControl) getValue(settings *mediaTrackSettings, value *float64) {
	*value = *(*settings).Pan
}

func (c *panControl) setValue(settings *mediaTrackSettings, value *float64) {
	(*settings).Pan = value
}

type saturationControl struct {
	defaultControl
}

func (c *saturationControl) getName() string {
	return "saturation"
}

func (c *saturationControl) getSettingRange(capabilities *mediaTrackCapabilities) *mediaSettingRange {
	return (*capabilities).Saturation
}

func (c *saturationControl) getValue(settings *mediaTrackSettings, value *float64) {
	*value = *(*settings).Saturation
}

func (c *saturationControl) setValue(settings *mediaTrackSettings, value *float64) {
	(*settings).Saturation = value
}

type sharpnessControl struct {
	defaultControl
}

func (c *sharpnessControl) getName() string {
	return "sharpness"
}

func (c *sharpnessControl) getSettingRange(capabilities *mediaTrackCapabilities) *mediaSettingRange {
	return (*capabilities).Sharpness
}

func (c *sharpnessControl) getValue(settings *mediaTrackSettings, value *float64) {
	*value = *(*settings).Sharpness
}

func (c *sharpnessControl) setValue(settings *mediaTrackSettings, value *float64) {
	(*settings).Sharpness = value
}

type tiltControl struct {
	defaultControl
}

func (c *tiltControl) getName() string {
	return "tilt"
}

func (c *tiltControl) getSettingRange(capabilities *mediaTrackCapabilities) *mediaSettingRange {
	return (*capabilities).Tilt
}

func (c *tiltControl) getValue(settings *mediaTrackSettings, value *float64) {
	*value = *(*settings).Tilt
}

func (c *tiltControl) setValue(settings *mediaTrackSettings, value *float64) {
	(*settings).Tilt = value
}

type zoomControl struct {
	defaultControl
}

func (c *zoomControl) getName() string {
	return "zoom"
}

func (c *zoomControl) getSettingRange(capabilities *mediaTrackCapabilities) *mediaSettingRange {
	return (*capabilities).Zoom
}

func (c *zoomControl) getValue(settings *mediaTrackSettings, value *float64) {
	*value = *(*settings).Zoom
}

func (c *zoomControl) setValue(settings *mediaTrackSettings, value *float64) {
	(*settings).Zoom = value
}

type exposureModeControl struct {
	defaultControl
}

func (c *exposureModeControl) getName() string {
	return "exposureMode"
}

type focusModeControl struct {
	defaultControl
}

func (c *focusModeControl) getName() string {
	return "focusMode"
}

type whiteBalanceModeControl struct {
	defaultControl
}

func (c *whiteBalanceModeControl) getName() string {
	return "whiteBalanceMode"
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
	testMediaControls := [...]mediaControl{
		&exposureTimeControl{},
		&focusDistanceControl{},
		&colorTemperatureControl{},
		&exposureCompensationControl{},
		&isoControl{},
		&brightnessControl{},
		&contrastControl{},
		&saturationControl{},
		&sharpnessControl{},
		&panControl{},
		&tiltControl{},
		&zoomControl{},
	}
	capabilities, err := getMediaTrackCapabilities(ctx, conn)
	if err != nil {
		s.Fatal("Can't get capabilities: ", err)
		return
	}
	for _, control := range testMediaControls {
		settingRangeAddr := control.getSettingRange(&capabilities)
		if settingRangeAddr == nil {
			// Skip unsupported control.
			continue
		}
		settingRange := *settingRangeAddr
		s.Logf("support %s Range max:%f, min:%f, step:%f", control.getName(), settingRange.Max, settingRange.Min, settingRange.Step)

		// verify invalid values
		control.initInvalidTestValues(settingRange)
		verifyControl(ctx, s, conn, control, false)

		// verify validvalues
		control.initValidTestValues(settingRange)
		verifyControl(ctx, s, conn, control, true)
	}
}

func verifyControl(ctx context.Context, s *testing.State, conn *chrome.Conn, control mediaControl, isValid bool) {
	var testValues []float64
	if isValid {
		testValues = control.getValidTestValues()
	} else {
		testValues = control.getInvalidTestValues()
	}
	for _, value := range testValues {
		var constraints mediaTrackConstraints
		settings, err := getMediaTrackSettings(ctx, conn)
		if err != nil {
			s.Fatal("Can't get settings: ", err)
		}
		var origValue float64
		control.getValue(&settings, &origValue)
		constraints.Advanced[0] = control.getPrerequisite()
		control.setValue(&constraints.Advanced[0], &value)
		err = applyMediaTrackConstraints(ctx, conn, constraints)
		if isValid && err != nil {
			s.Fatal("Can't apply constraints: ", err)
		}
		settings, err = getMediaTrackSettings(ctx, conn)
		if err != nil {
			s.Fatal("Can't get settings: ", err)
		}
		var getValue float64
		control.getValue(&settings, &getValue)
		differance := math.Abs(getValue - value)
		if isValid && differance > control.getTolerance() {
			s.Errorf("Set %s to %f, get %f, differance %f, tolerance %f", control.getName(), value, getValue, differance, control.getTolerance())
		}
		if !isValid && getValue != origValue {
			s.Errorf("%s: Get value %f != Original value %f", control.getName(), getValue, origValue)
		}
	}
}
