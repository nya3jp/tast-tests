// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package settings implements a library used for communication with Chrome settings.
// A chrome.Conn with the "settingsPrivate" permission is needed, like the one returned by TestAPIConn().
package settings

import (
	"context"
	"fmt"

	"chromiumos/tast/local/chrome"
)

// DefaultZoom returns the default page zoom factor. Possible values are currently between
// 0.25 and 5. For a full list, see zoom::kPresetZoomFactors in:
// https://cs.chromium.org/chromium/src/components/zoom/page_zoom_constants.cc
func DefaultZoom(ctx context.Context, c *chrome.Conn) (float64, error) {
	var zoom float64
	if err := c.EvalPromise(ctx,
		`new Promise(function(resolve, reject) {
		  chrome.settingsPrivate.getDefaultZoom(function(zoom) {
		    if (chrome.runtime.lastError) {
		      reject(new Error(chrome.runtime.lastError.message));
		      return;
		    }
		    resolve(zoom);
		  })
		})`, &zoom); err != nil {
		return 0, err
	}
	return zoom, nil
}

// SetDefaultZoom sets the page zoom factor. Must be less than 0.001 different than a value
// in zoom::kPresetZoomFactors. See:
// https://cs.chromium.org/chromium/src/components/zoom/page_zoom_constants.cc
func SetDefaultZoom(ctx context.Context, c *chrome.Conn, zoom float64) error {
	expr := fmt.Sprintf(
		`new Promise(function(resolve, reject) {
		  chrome.settingsPrivate.setDefaultZoom(%f, function(success) {
		    if (chrome.runtime.lastError) {
		      reject(new Error(chrome.runtime.lastError.message));
		      return;
		    }
		    if (!success) {
		      reject(new Error("setDefaultZoom() failed"));
		      return;
		    }
		    resolve();
		  })
		})`, zoom)
	return c.EvalPromise(ctx, expr, nil)
}

const (
	// NightLightScheduleNever means Night Light is never enabled.
	NightLightScheduleNever = 0
	// NightLightScheduleSunsetToSunrise means Night Light is enabled at night.
	NightLightScheduleSunsetToSunrise = 1
	// NightLightScheduleCustom means Night Light has a custom schedule.
	NightLightScheduleCustom = 2
)

// NightLightSchedule gets the current Night Light schedule. See the above
// constants for possible values.
func NightLightSchedule(ctx context.Context, c *chrome.Conn) (uint, error) {
	var schedule uint
	if err := c.EvalPromise(ctx,
		`new Promise(function(resolve, reject) {
		  chrome.settingsPrivate.getPref("ash.night_light.schedule_type", function(schedule) {
		    if (chrome.runtime.lastError) {
		      reject(new Error(chrome.runtime.lastError.message));
		      return;
		    }
		    resolve(schedule.value);
		  })
		})`, &schedule); err != nil {
		return 0, err
	}
	return schedule, nil
}

// SetNightLightSchedule sets the current Night Light schedule.
func SetNightLightSchedule(ctx context.Context, c *chrome.Conn, schedule uint) error {
	expr := fmt.Sprintf(
		`new Promise(function(resolve, reject) {
		  chrome.settingsPrivate.setPref("ash.night_light.schedule_type", %d, function(success) {
		    if (chrome.runtime.lastError) {
		      reject(new Error(chrome.runtime.lastError.message));
		      return;
		    }
		    if (!success) {
		      reject(new Error("set ash.night_light.schedule_type failed"));
		      return;
		    }
		    resolve();
		  })
		})`, schedule)
	return c.EvalPromise(ctx, expr, nil)
}

// NightLightEnabled returns true if Night Light is currently enabled.
func NightLightEnabled(ctx context.Context, c *chrome.Conn) (bool, error) {
	var enabled bool
	if err := c.EvalPromise(ctx,
		`new Promise(function(resolve, reject) {
		  chrome.settingsPrivate.getPref("ash.night_light.enabled", function(enabled) {
		    if (chrome.runtime.lastError) {
		      reject(new Error(chrome.runtime.lastError.message));
		      return;
		    }
		    resolve(enabled.value);
		  })
		})`, &enabled); err != nil {
		return false, err
	}
	return enabled, nil
}

// SetNightLightEnabled enables or disables Night Light.
func SetNightLightEnabled(ctx context.Context, c *chrome.Conn, enabled bool) error {
	expr := fmt.Sprintf(
		`new Promise(function(resolve, reject) {
		  chrome.settingsPrivate.setPref("ash.night_light.enabled", %t, function(success) {
		    if (chrome.runtime.lastError) {
		      reject(new Error(chrome.runtime.lastError.message));
		      return;
		    }
		    if (!success) {
		      reject(new Error("set ash.night_light.enabled failed"));
		      return;
		    }
		    resolve();
		  })
		})`, enabled)
	return c.EvalPromise(ctx, expr, nil)
}
