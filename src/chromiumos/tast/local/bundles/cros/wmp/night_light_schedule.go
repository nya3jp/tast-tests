// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wmp

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/settings"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/testing"
)

// timeData stores the time information hour:minute in 24 hours format.
type timeData struct {
	hour   int
	minute int
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         NightLightSchedule,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests the adjustment of night light schedule",
		Contacts: []string{
			"zxdan@chromium.org",
			"chromeos-wmp@google.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
	})
}

// NightLightSchedule tests the adjustment of night light schedule.
func NightLightSchedule(ctx context.Context, s *testing.State) {
	// Reserve five seconds for various cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure clamshell mode: ", err)
	}
	defer cleanup(cleanupCtx)

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree_night_light_schedule")

	ui := uiauto.New(tconn)

	// Turn on night light by clicking the night light pod icon in quick settings.
	nightLightIconButton := nodewith.HasClass("FeaturePodIconButton").NameContaining("Night Light")
	if err := uiauto.Combine(
		"Click night light pod in quick settings",
		ui.LeftClick(nodewith.HasClass("UnifiedSystemTray")),
		ui.LeftClick(nightLightIconButton),
	)(ctx); err != nil {
		s.Fatal("Failed to click night light pod in quick settings: ", err)
	}

	nightLightEnabled, err := settings.NightLightEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get night light enabled state: ", err)
	}

	if !nightLightEnabled {
		s.Fatal("Failed to enable night light by clicking pod button")
	}

	nightLightIconButtonInfo, err := ui.Info(ctx, nightLightIconButton)
	if err != nil {
		s.Fatal("Failed to get info from night light icon button: ", err)
	}

	if strings.Contains(nightLightIconButtonInfo.Name, "Night Light is off") {
		s.Logf("Night Light Pod Button Name: %s", nightLightIconButtonInfo.Name)
		s.Fatal("After clicking night light pod button, it still shows \"Night light is off\"")
	}

	// Open display settings by clicking the sub label of night light pod.
	if err := uiauto.Combine(
		"Show display settings by clicking night light sub label",
		ui.LeftClick(nodewith.HasClass("FeaturePodLabelButton").Name("Show display settings")),
		ui.WaitUntilExists(nodewith.HasClass("BrowserFrame").Name("Settings - Displays")),
	)(ctx); err != nil {
		s.Fatal("Failed to click night light sub label to show display settings: ", err)
	}

	// Change night light schedule to custom in drop box.
	customScheduleOption := nodewith.Role("listBoxOption").Name("Custom")
	startTimeKnob := nodewith.HasClass("knob").NameContaining("Start time")
	endTimeKnob := nodewith.HasClass("knob").NameContaining("End time")
	if err := uiauto.Combine(
		"Choose custom night light schedule",
		ui.LeftClick(nodewith.Role("popUpButton").Name("Schedule")),
		ui.LeftClick(customScheduleOption),
		ui.WaitUntilExists(startTimeKnob),
	)(ctx); err != nil {
		s.Fatal("Failed to change night light schedule to custom: ", err)
	}

	// Check if night light is on when current time is within the schedule,
	// or is off when current time is outside schedule.
	// Get start time from start knob.
	startTimeKnobInfo, err := ui.Info(ctx, startTimeKnob)
	if err != nil {
		s.Fatal("Failed to get info from start time knob: ", err)
	}

	startTime, err := extractTimeFromString(ctx, startTimeKnobInfo.Name)
	if err != nil {
		s.Fatal("Failed to get time from srart time knob: ", err)
	}

	// Get end time from end knob.
	endTimeKnobInfo, err := ui.Info(ctx, endTimeKnob)
	if err != nil {
		s.Fatal("Failed to get info from end time knob: ", err)
	}

	endTime, err := extractTimeFromString(ctx, endTimeKnobInfo.Name)
	if err != nil {
		s.Fatal("Failed to get time from end time knob: ", err)
	}

	// If end time is earlier than start time, add 24 hours to the end time..
	if compareTime(endTime, startTime) < 0 {
		endTime.hour += 24
	}

	// Get current time from time view.
	timeViewInfo, err := ui.Info(ctx, nodewith.HasClass("TimeView").First())
	if err != nil {
		s.Fatal("Failed to get info from time view: ", err)
	}

	currentTime, err := extractTimeFromString(ctx, timeViewInfo.Name)
	if err != nil {
		s.Fatal("Failed to get time from time view: ", err)
	}

	// Check if current time is in the schedule.
	inSchedule := compareTime(currentTime, startTime) >= 0 && compareTime(currentTime, endTime) < 0

	// Get current night light enabled state.
	nightLightEnabled, err = settings.NightLightEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get night light enabled state: ", err)
	}

	// Check if night light is on/off according to if current time is in/out schedule.
	if inSchedule && !nightLightEnabled {
		s.Fatal("Night light should be on when current time is in schedule")
	} else if !inSchedule && nightLightEnabled {
		s.Fatal("Night light should be off when current time is not in schedule")
	}
}

// extractTimeFromString extracts the time from the given string and returns the time data in 24 hours format.
func extractTimeFromString(ctx context.Context, stringWithTime string) (timeData, error) {
	// Get time string with format HH:MM A/PM from string.
	reg := regexp.MustCompile("\\d+:\\d\\d\\s[A|P]M")
	timeString := reg.FindString(stringWithTime)

	data := timeData{hour: 0, minute: 0}
	period := "AM"
	_, err := fmt.Sscanf(timeString, "%d:%d %s", &data.hour, &data.minute, &period)
	if err != nil {
		return data, errors.Wrapf(err, "failed to get time with format HH:MM A/PM from %s", timeString)
	}

	if period == "PM" && data.hour != 12 {
		data.hour += 12
	}

	return data, nil
}

// compareTime compares time1 with time2 and returns -1 if the former is earlier, 0 if they are equal, 1 if the former is latter.
func compareTime(time1, time2 timeData) int {
	if time1.hour != time2.hour {
		if time1.hour-time2.hour > 0 {
			return 1
		}
		return -1
	}

	if time1.minute != time2.minute {
		if time1.minute-time2.minute > 0 {
			return 1
		}
		return -1
	}

	return 0
}
