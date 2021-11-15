// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package settings implements a library used for communication with Chrome settings.
// A chrome.TestConn returned by TestAPIConn() with the "settingsPrivate" permission is needed.
package settings

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
)

// NightLightScheduleValue provides available values for the Night Light
// schedule.
type NightLightScheduleValue uint

// The following constants are from NightLightController::ScheduleType in
// chromium/src/ash/public/cpp/night_light_controller.h.
const (
	// NightLightScheduleNever means Night Light is never enabled.
	NightLightScheduleNever NightLightScheduleValue = 0
	// NightLightScheduleSunsetToSunrise means Night Light is enabled at night.
	NightLightScheduleSunsetToSunrise NightLightScheduleValue = 1
	// NightLightScheduleCustom means Night Light has a custom schedule.
	NightLightScheduleCustom NightLightScheduleValue = 2
)

// The following are corresponding pref settings.
const (
	nightLightSchedulePref           = "ash.night_light.schedule_type"
	nightLightEnabledPref            = "ash.night_light.enabled"
	trackpadReverseScrollEnabledPref = "settings.touchpad.natural_scroll"
)

// DefaultZoom returns the default page zoom factor. Possible values are currently between
// 0.25 and 5. For a full list, see zoom::kPresetZoomFactors in:
// https://cs.chromium.org/chromium/src/components/zoom/page_zoom_constants.cc
func DefaultZoom(ctx context.Context, tconn *chrome.TestConn) (float64, error) {
	var zoom float64
	if err := tconn.Call(ctx, &zoom, "tast.promisify(chrome.settingsPrivate.getDefaultZoom)"); err != nil {
		return 0, err
	}
	return zoom, nil
}

// SetDefaultZoom sets the page zoom factor. Must be less than 0.001 different than a value
// in zoom::kPresetZoomFactors. See:
// https://cs.chromium.org/chromium/src/components/zoom/page_zoom_constants.cc
func SetDefaultZoom(ctx context.Context, tconn *chrome.TestConn, zoom float64) error {
	return tconn.Call(ctx, nil, `async (zoom) => {
	  if (!await tast.promisify(chrome.settingsPrivate.setDefaultZoom)(zoom))
	    throw new Error("setDefaultZoom() failed");
	}`, zoom)
}

// NightLightSchedule gets the current Night Light schedule. See the above
// constants for possible values.
func NightLightSchedule(ctx context.Context, c *chrome.TestConn) (NightLightScheduleValue, error) {
	var schedule struct {
		Value uint `json:"value"`
	}
	if err := c.Call(ctx, &schedule, "tast.promisify(chrome.settingsPrivate.getPref)", nightLightSchedulePref); err != nil {
		return 0, err
	}
	switch schedule.Value {
	case uint(NightLightScheduleNever):
		return NightLightScheduleNever, nil
	case uint(NightLightScheduleSunsetToSunrise):
		return NightLightScheduleSunsetToSunrise, nil
	case uint(NightLightScheduleCustom):
		return NightLightScheduleCustom, nil
	default:
		return 0, errors.Errorf("unrecognized Night Light schedule %d", schedule)
	}
}

// SetNightLightSchedule sets the current Night Light schedule.
func SetNightLightSchedule(ctx context.Context, c *chrome.TestConn, schedule NightLightScheduleValue) error {
	return c.Call(ctx, nil, "tast.promisify(chrome.settingsPrivate.setPref)", nightLightSchedulePref, schedule)
}

// NightLightEnabled returns true if Night Light is currently enabled.
func NightLightEnabled(ctx context.Context, c *chrome.TestConn) (bool, error) {
	var enabled struct {
		Value bool `json:"value"`
	}
	if err := c.Call(ctx, &enabled, "tast.promisify(chrome.settingsPrivate.getPref)", nightLightEnabledPref); err != nil {
		return false, err
	}
	return enabled.Value, nil
}

// SetNightLightEnabled enables or disables Night Light.
func SetNightLightEnabled(ctx context.Context, c *chrome.TestConn, enabled bool) error {
	return c.Call(ctx, nil, "tast.promisify(chrome.settingsPrivate.setPref)", nightLightEnabledPref, enabled)
}

// TrackpadReverseScrollEnabled gets current enabled state of trackpad reverse scrolling.
func TrackpadReverseScrollEnabled(ctx context.Context, c *chrome.TestConn) (bool, error) {
	var enabled struct {
		Value bool `json:"value"`
	}
	if err := c.Call(ctx, &enabled, "tast.promisify(chrome.settingsPrivate.getPref)", trackpadReverseScrollEnabledPref); err != nil {
		return false, err
	}
	return enabled.Value, nil
}

// SetTrackpadReverseScrollEnabled enables/disables trackpad reverse scrolling..
func SetTrackpadReverseScrollEnabled(ctx context.Context, c *chrome.TestConn, enabled bool) error {
	return c.Call(ctx, nil, "tast.promisify(chrome.settingsPrivate.setPref)", trackpadReverseScrollEnabledPref, enabled)
}
