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
	"strings"

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
	// getSettingRange returns nil if the platform doesn't support such control.
	getSettingRange(*mediaTrackCapabilities) *mediaSettingRange
	getValue(*mediaTrackSettings) float64
	getTolerance() float64
	isEqual(float64, float64) bool
	getValidTestValues(mediaSettingRange) []float64
	getInvalidTestValues(mediaSettingRange) []float64
	// getConstraints gets constraint for setting |value| to control |c|.
	getConstraints(value *float64) mediaTrackConstraints
	getDefaultConstraints(mediaSettingRange) mediaTrackConstraints
}

type defaultControl struct {
	mediaControl
}

func (c *defaultControl) getTolerance() float64 {
	return 0.0
}

func (c *defaultControl) isEqual(v1, v2 float64) bool {
	differance := math.Abs(v1 - v2)
	return differance <= c.getTolerance()
}

func (c *defaultControl) getValidTestValues(r mediaSettingRange) []float64 {
	totalSteps := (r.Max - r.Min) / r.Step
	middleValue := r.Min + r.Step*math.Round(totalSteps/2)
	return []float64{r.Min, middleValue, r.Max}
}

func (c *defaultControl) getInvalidTestValues(r mediaSettingRange) []float64 {
	return []float64{r.Min - r.Step, r.Max + r.Step}
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

func (c *brightnessControl) getValue(settings *mediaTrackSettings) float64 {
	return *(*settings).Brightness
}

func (c *brightnessControl) getConstraints(value *float64) mediaTrackConstraints {
	return mediaTrackConstraints{
		Advanced: [1]mediaTrackSettings{
			mediaTrackSettings{Brightness: value},
		},
	}
}

func (c *brightnessControl) getDefaultConstraints(r mediaSettingRange) mediaTrackConstraints {
	totalSteps := (r.Max - r.Min) / r.Step
	middleValue := r.Min + r.Step*math.Round(totalSteps/2)
	return c.getConstraints(&middleValue)
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

func (c *colorTemperatureControl) getValue(settings *mediaTrackSettings) float64 {
	return *(*settings).ColorTemperature
}

func (c *colorTemperatureControl) getValidTestValues(r mediaSettingRange) []float64 {
	// We need to use specified values for color temperatures, because it is translated from white balance mode.
	availableColorTemperatures := []float64{2700, 3000, 5000, 5500, 6500, 7500, 15000}
	var validTestValues []float64
	for _, value := range availableColorTemperatures {
		if value >= r.Min && value <= r.Max {
			validTestValues = append(validTestValues, value)
		}
	}
	return validTestValues
}

func (c *colorTemperatureControl) getConstraints(value *float64) mediaTrackConstraints {
	manual := "manual"
	return mediaTrackConstraints{
		Advanced: [1]mediaTrackSettings{
			mediaTrackSettings{WhiteBalanceMode: &manual, ColorTemperature: value},
		},
	}
}

func (c *colorTemperatureControl) getDefaultConstraints(r mediaSettingRange) mediaTrackConstraints {
	continuous := "continuous"
	return mediaTrackConstraints{
		Advanced: [1]mediaTrackSettings{
			mediaTrackSettings{WhiteBalanceMode: &continuous},
		},
	}
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

func (c *contrastControl) getValue(settings *mediaTrackSettings) float64 {
	return *(*settings).Contrast
}

func (c *contrastControl) getConstraints(value *float64) mediaTrackConstraints {
	return mediaTrackConstraints{
		Advanced: [1]mediaTrackSettings{
			mediaTrackSettings{Contrast: value},
		},
	}
}

func (c *contrastControl) getDefaultConstraints(r mediaSettingRange) mediaTrackConstraints {
	totalSteps := (r.Max - r.Min) / r.Step
	middleValue := r.Min + r.Step*math.Round(totalSteps/2)
	return c.getConstraints(&middleValue)
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

func (c *exposureCompensationControl) getValue(settings *mediaTrackSettings) float64 {
	return *(*settings).ExposureCompensation
}

func (c *exposureCompensationControl) getConstraints(value *float64) mediaTrackConstraints {
	continuous := "continuous"
	return mediaTrackConstraints{
		Advanced: [1]mediaTrackSettings{
			mediaTrackSettings{ExposureMode: &continuous, ExposureCompensation: value},
		},
	}
}

func (c *exposureCompensationControl) getDefaultConstraints(r mediaSettingRange) mediaTrackConstraints {
	totalSteps := (r.Max - r.Min) / r.Step
	middleValue := r.Min + r.Step*math.Round(totalSteps/2)
	return c.getConstraints(&middleValue)
}

type exposureTimeControl struct {
	defaultControl
}

func (c *exposureTimeControl) getName() string {
	return "exposureTime"
}

func (c *exposureTimeControl) getValue(settings *mediaTrackSettings) float64 {
	return *(*settings).ExposureTime
}

func (c *exposureTimeControl) getSettingRange(capabilities *mediaTrackCapabilities) *mediaSettingRange {
	return (*capabilities).ExposureTime
}

func (c *exposureTimeControl) getTolerance() float64 {
	return 1.0
}

func (c *exposureTimeControl) isEqual(v1, v2 float64) bool {
	differance := math.Abs(v1 - v2)
	return differance <= c.getTolerance()
}

func (c *exposureTimeControl) getConstraints(value *float64) mediaTrackConstraints {
	manual := "manual"
	return mediaTrackConstraints{
		Advanced: [1]mediaTrackSettings{
			mediaTrackSettings{ExposureMode: &manual, ExposureTime: value},
		},
	}
}

func (c *exposureTimeControl) getDefaultConstraints(r mediaSettingRange) mediaTrackConstraints {
	continuous := "continuous"
	return mediaTrackConstraints{
		Advanced: [1]mediaTrackSettings{
			mediaTrackSettings{ExposureMode: &continuous},
		},
	}
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

func (c *focusDistanceControl) getValue(settings *mediaTrackSettings) float64 {
	return *(*settings).FocusDistance
}

func (c *focusDistanceControl) getTolerance() float64 {
	return 0.01
}

func (c *focusDistanceControl) isEqual(v1, v2 float64) bool {
	differance := math.Abs(v1 - v2)
	return differance <= c.getTolerance()
}

func (c *focusDistanceControl) getValidTestValues(r mediaSettingRange) []float64 {
	// Only min values of focusDistances meaningful.
	return []float64{r.Min, r.Min + r.Step}
}

func (c *focusDistanceControl) getConstraints(value *float64) mediaTrackConstraints {
	manual := "manual"
	return mediaTrackConstraints{
		Advanced: [1]mediaTrackSettings{
			mediaTrackSettings{FocusMode: &manual, FocusDistance: value},
		},
	}
}

func (c *focusDistanceControl) getDefaultConstraints(r mediaSettingRange) mediaTrackConstraints {
	continuous := "continuous"
	return mediaTrackConstraints{
		Advanced: [1]mediaTrackSettings{
			mediaTrackSettings{FocusMode: &continuous},
		},
	}
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

func (c *isoControl) getValue(settings *mediaTrackSettings) float64 {
	return *(*settings).Iso
}

func (c *isoControl) getTolerance() float64 {
	return 5.0
}

func (c *isoControl) isEqual(v1, v2 float64) bool {
	differance := math.Abs(v1 - v2)
	return differance <= c.getTolerance()
}

func (c *isoControl) getConstraints(value *float64) mediaTrackConstraints {
	manual := "manual"
	exposureTime333 := 333.0
	return mediaTrackConstraints{
		Advanced: [1]mediaTrackSettings{
			mediaTrackSettings{ExposureMode: &manual, ExposureTime: &exposureTime333, Iso: value},
		},
	}
}

func (c *isoControl) getDefaultConstraints(r mediaSettingRange) mediaTrackConstraints {
	continuous := "continuous"
	return mediaTrackConstraints{
		Advanced: [1]mediaTrackSettings{
			mediaTrackSettings{ExposureMode: &continuous},
		},
	}
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

func (c *panControl) getConstraints(value *float64) mediaTrackConstraints {
	return mediaTrackConstraints{
		Advanced: [1]mediaTrackSettings{
			mediaTrackSettings{Pan: value},
		},
	}
}

func (c *panControl) getDefaultConstraints(r mediaSettingRange) mediaTrackConstraints {
	totalSteps := (r.Max - r.Min) / r.Step
	middleValue := r.Min + r.Step*math.Round(totalSteps/2)
	return c.getConstraints(&middleValue)
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

func (c *saturationControl) getValue(settings *mediaTrackSettings) float64 {
	return *(*settings).Saturation
}

func (c *saturationControl) getConstraints(value *float64) mediaTrackConstraints {
	return mediaTrackConstraints{
		Advanced: [1]mediaTrackSettings{
			mediaTrackSettings{Saturation: value},
		},
	}
}

func (c *saturationControl) getDefaultConstraints(r mediaSettingRange) mediaTrackConstraints {
	totalSteps := (r.Max - r.Min) / r.Step
	middleValue := r.Min + r.Step*math.Round(totalSteps/2)
	return c.getConstraints(&middleValue)
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

func (c *sharpnessControl) getValue(settings *mediaTrackSettings) float64 {
	return *(*settings).Sharpness
}

func (c *sharpnessControl) getConstraints(value *float64) mediaTrackConstraints {
	return mediaTrackConstraints{
		Advanced: [1]mediaTrackSettings{
			mediaTrackSettings{Sharpness: value},
		},
	}
}

func (c *sharpnessControl) getDefaultConstraints(r mediaSettingRange) mediaTrackConstraints {
	totalSteps := (r.Max - r.Min) / r.Step
	middleValue := r.Min + r.Step*math.Round(totalSteps/2)
	return c.getConstraints(&middleValue)
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

func (c *tiltControl) getValue(settings *mediaTrackSettings) float64 {
	return *(*settings).Tilt
}

func (c *tiltControl) getConstraints(value *float64) mediaTrackConstraints {
	return mediaTrackConstraints{
		Advanced: [1]mediaTrackSettings{
			mediaTrackSettings{Tilt: value},
		},
	}
}

func (c *tiltControl) getDefaultConstraints(r mediaSettingRange) mediaTrackConstraints {
	totalSteps := (r.Max - r.Min) / r.Step
	middleValue := r.Min + r.Step*math.Round(totalSteps/2)
	return c.getConstraints(&middleValue)
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

func (c *zoomControl) getValue(settings *mediaTrackSettings) float64 {
	return *(*settings).Zoom
}

func (c *zoomControl) getConstraints(value *float64) mediaTrackConstraints {
	return mediaTrackConstraints{
		Advanced: [1]mediaTrackSettings{
			mediaTrackSettings{Zoom: value},
		},
	}
}

func (c *zoomControl) getDefaultConstraints(r mediaSettingRange) mediaTrackConstraints {
	totalSteps := (r.Max - r.Min) / r.Step
	middleValue := r.Min + r.Step*math.Round(totalSteps/2)
	return c.getConstraints(&middleValue)
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
		s.Fatal("Failed to getCameras: ", err)
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

		// Verify invalid values.
		verifyControl(ctx, s, conn, control, settingRange, false)

		// Verify validvalues.
		verifyControl(ctx, s, conn, control, settingRange, true)
	}
}

func verifyControl(ctx context.Context, s *testing.State, conn *chrome.Conn, control mediaControl, r mediaSettingRange, isValid bool) {
	var testValues []float64
	if isValid {
		testValues = control.getValidTestValues(r)
	} else {
		testValues = control.getInvalidTestValues(r)
	}
	for _, value := range testValues {
		settings, err := getMediaTrackSettings(ctx, conn)
		if err != nil {
			s.Fatal("Can't get settings: ", err)
		}
		var origValue float64
		origValue = control.getValue(&settings)
		err = applyMediaTrackConstraints(ctx, conn, control.getConstraints(&value))
		if err != nil {
			if isValid || !strings.Contains(err.Error(), "out of range") {
				s.Fatal("Can't apply constraints: ", err)
			}
		}
		settings, err = getMediaTrackSettings(ctx, conn)
		if err != nil {
			s.Fatal("Can't get settings: ", err)
		}
		var getValue float64
		getValue = control.getValue(&settings)
		if isValid {
			if !control.isEqual(value, getValue) {
				s.Errorf("Failed to apply %v control, want %v; got %v with tolerance %v", control.getName(), value, getValue, control.getTolerance())
			}
		} else {
			// For not valid case, the origValue is not change.
			if origValue != getValue {
				s.Errorf("Invalid %v control changed after applied, want %v; got %v", control.getName(), origValue, getValue)
			}
		}
		err = applyMediaTrackConstraints(ctx, conn, control.getDefaultConstraints(r))
		if err != nil {
			s.Fatal("Can't apply default constraints: ", err)
		}
	}
}
