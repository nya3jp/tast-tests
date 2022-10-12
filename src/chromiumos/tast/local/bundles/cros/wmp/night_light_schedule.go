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

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree_rename_desk")

	ui := uiauto.New(tconn)

	// Turn on night light by clicking the night light pod icon in quick settings.
	nightLightIconButton := nodewith.ClassName("FeaturePodIconButton").NameContaining("Night Light")
	if err := uiauto.Combine(
		"Click night light pod in quick settings",
		ui.LeftClick(nodewith.HasClass("UnifiedSystemTray")),
		ui.WaitUntilExists(nightLightIconButton),
		ui.LeftClick(nightLightIconButton),
	)(ctx); err != nil {
		s.Fatal("Failed to click night light pod in quick settings: ", err)
	}

	nightLightIconButtonInfo, err := ui.Info(ctx, nightLightIconButton)
	if err != nil {
		s.Fatal("Failed to get info from night light icon button: ", err)
	}

	if !strings.Contains(nightLightIconButtonInfo.Name, "Night Light is on") {
		s.Fatal("Failed to turn on the night light by clicking pod button")
	}

	// Open display settings by clicking the sub label of night light pod.
	if err := uiauto.Combine(
		"Show display settings by clicking night light sub label",
		ui.LeftClick(nodewith.ClassName("FeaturePodLabelButton").Name("Show display settings")),
		ui.WaitUntilExists(nodewith.ClassName("BrowserFrame").Name("Settings - Displays")),
	)(ctx); err != nil {
		s.Fatal("Failed to click night light sub label to show display settings: ", err)
	}

	// Change night light schedule to custom in drop box.
	customScheduleOption := nodewith.Role("listBoxOption").Name("Custom")
	startTimeKnob := nodewith.ClassName("knob").NameContaining("Start time")
	endTimeKnob := nodewith.ClassName("knob").NameContaining("End time")
	if err := uiauto.Combine(
		"Choose custom night light schedule",
		ui.LeftClick(nodewith.Role("popUpButton").Name("Schedule")),
		ui.WaitUntilExists(customScheduleOption),
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

	startTime, err := getTimeFromString(ctx, startTimeKnobInfo.Name)
	if err != nil {
		s.Fatal("Failed to get time from srart time knob: ", err)
	}

	// Get end time from end knob.
	endTimeKnobInfo, err := ui.Info(ctx, endTimeKnob)
	if err != nil {
		s.Fatal("Failed to get info from end time knob: ", err)
	}

	endTime, err := getTimeFromString(ctx, endTimeKnobInfo.Name)
	if err != nil {
		s.Fatal("Failed to get time from end time knob: ", err)
	}

	// Get current time from time view.
	timeViewInfo, err := ui.Info(ctx, nodewith.ClassName("TimeView").First())
	if err != nil {
		s.Fatal("Failed to get info from time view: ", err)
	}

	currentTime, err := getTimeFromString(ctx, timeViewInfo.Name)
	if err != nil {
		s.Fatal("Failed to get time from time view: ", err)
	}

	// Check if current time is in the schedule.
	nightLightOn := false
	if compareTime(startTime, endTime) < 0 {
		nightLightOn = timeInRange(currentTime, startTime, endTime, false)
	} else {
		midNight := timeData{hour: 24, minute: 0}
		earlyMorning := timeData{hour: 0, minute: 0}
		inStartToNight := timeInRange(currentTime, startTime, midNight, true)
		inMorningToEnd := timeInRange(currentTime, earlyMorning, endTime, false)
		nightLightOn = inStartToNight || inMorningToEnd
	}

	// If night light is on, there should be color temperature options.
	colorTempNodesInfo, err := ui.NodesInfo(ctx, nodewith.Name("Color temperature"))
	if err != nil {
		s.Fatal("Failed to get color temperature nodes info: ", err)
	}

	if nightLightOn && len(colorTempNodesInfo) == 0 {
		s.Fatal("Night light should be turned on")
	} else if !nightLightOn && len(colorTempNodesInfo) > 0 {
		s.Fatal("Night light should be turned off")
	}
}

// getTimeFromString extracts the time from the given string and returns the time data in 24 hours format.
func getTimeFromString(ctx context.Context, stringWithTime string) (timeData, error) {
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

// compareTime compares time1 with time2 and returns negative if the former is earlier, 0 if they are equal, positive if the former is latter.
func compareTime(time1, time2 timeData) int {
	if time1.hour != time2.hour {
		return time1.hour - time2.hour
	}

	if time1.minute != time2.minute {
		return time1.minute - time2.minute
	}

	return 0
}

// timeInRange returns true if the target time is in a given range from start time to end time.
func timeInRange(targetTime, startTime, endTime timeData, includeEnd bool) bool {
	if compareTime(startTime, endTime) > 0 {
		return false
	}

	if compareTime(targetTime, startTime) < 0 {
		return false
	}

	compareEnd := compareTime(targetTime, endTime)
	if compareEnd < 0 || (includeEnd && compareEnd == 0) {
		return true
	}

	return false
}
