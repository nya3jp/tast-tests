// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wmp

import (
	"context"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         NightLightColorTemperature,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests the adjustment of night light color temperature",
		Contacts: []string{
			"zxdan@chromium.org",
			"chromeos-wmp@google.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
	})
}

// NightLightColorTemperature tests the adjustment of night light color temperature.
func NightLightColorTemperature(ctx context.Context, s *testing.State) {
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

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree_night_light_color_temp")

	ui := uiauto.New(tconn)

	// Turn on night light and open display settings by clicking pod button in quick settings.
	nightLightIconButton := nodewith.HasClass("FeaturePodIconButton").NameContaining("Night Light")
	if err := uiauto.Combine(
		"Click night light pod button and its sub label in quick settings",
		ui.LeftClick(nodewith.HasClass("UnifiedSystemTray")),
		ui.LeftClick(nightLightIconButton),
		ui.LeftClick(nodewith.HasClass("FeaturePodLabelButton").Name("Show display settings")),
		ui.WaitUntilExists(nodewith.HasClass("BrowserFrame").Name("Settings - Displays")),
	)(ctx); err != nil {
		s.Fatal("Failed to enable night light and open display settings by clicking pod button in quick settings: ", err)
	}

	// Get current CTM (Color Transform Matrix).
	defaultCTM, err := modetestCTM(ctx)
	if err != nil {
		s.Fatal("Failed to get the CTM of default temperature from modetest: ", err)
	}

	colorTempContainer := nodewith.Role("genericContainer").Name("Color temperature")
	colorTempSliderInfo, err := ui.Info(ctx, nodewith.Role("slider").Ancestor(colorTempContainer))
	if err != nil {
		s.Fatal("Failed to get color temperature slider info: ", err)
	}
	sliderBounds := colorTempSliderInfo.Location

	// Drag slider to cooler side.
	if err := uiauto.Combine(
		"Drag the color temperature slider",
		mouse.Drag(tconn, sliderBounds.CenterPoint(), sliderBounds.LeftCenter(), time.Second*1),
	)(ctx); err != nil {
		s.Fatal("Failed to drag the color temperature slider to cooler side: ", err)
	}

	// Check if the color transform matrix changed.
	coolerCTM, err := modetestCTM(ctx)
	if err != nil {
		s.Fatal("Failed to get the CTM of cooler temperature from modetest: ", err)
	}

	if coolerCTM == defaultCTM {
		s.Fatal("Dragging slider to cooler does not change the night light color")
	}

	// Drag slider to warmer side.
	if err := uiauto.Combine(
		"Drag the color temperature slider",
		mouse.Drag(tconn, sliderBounds.LeftCenter(), sliderBounds.RightCenter(), time.Second*2),
	)(ctx); err != nil {
		s.Fatal("Failed to drag the color temperature slider to warmer side: ", err)
	}

	// Check if the color transform matrix changed.
	warmerCTM, err := modetestCTM(ctx)
	if err != nil {
		s.Fatal("Failed to get the CTM of warmer temperature from modetest: ", err)
	}

	if coolerCTM == warmerCTM || defaultCTM == warmerCTM {
		s.Fatal("Dragging slider to warmer does not change the night light color")
	}

	// Turn off night light by clicking the toggle button.
	if err := uiauto.Combine(
		"Turn off night light and color temperature section disappears",
		ui.LeftClick(nodewith.Role("toggleButton").Name("Night Light")),
		ui.WaitUntilGone(colorTempContainer),
	)(ctx); err != nil {
		s.Fatal("Failed to turn off the night light and make color temperature section disappear: ", err)
	}
}

// modetestCTM extracts the first non-empty CTM (color Transform Matrix) from the output of modetest.
func modetestCTM(ctx context.Context) (string, error) {
	output, err := testexec.CommandContext(ctx, "modetest", "-p").Output()
	if err != nil {
		return "", errors.Wrap(err, "failed to get the modetest output")
	}

	// The CTM info has the form:
	//  24 CTM:
	//	flags: ...
	//	bolbs: ...
	//	...
	//	values:
	//		001...00
	//		010...11
	//		...
	// 25 GAMMA_LUT:
	// The CTM contents are the numbers in value.
	ctmPattern := regexp.MustCompile("24\\sCTM:")
	ctmValuePattern := regexp.MustCompile("value:")
	ctmContentPattern := regexp.MustCompile("\\d+")
	ctmEndPattern := regexp.MustCompile("25\\sGAMMA_LUT:")

	findCTM := 1
	findCTMValue := 2
	findCTMContent := 3

	// Start to find "24 CTM" line.
	find := findCTM
	ctm := ""
	for _, line := range strings.Split(string(output), "\n") {
		if find == findCTM {
			// If found "24 CTM:" line, continue to find "value:" line.
			if ctmPattern.MatchString(line) {
				find++
			}
		} else if find == findCTMValue {
			// If found "value:" line, continue to find the content lines.
			if ctmValuePattern.MatchString(line) {
				find++
			}
		} else if find == findCTMContent {
			// If encounter "25 GAMMA_LUT" line, break when we get a non-empty CTM,
			// otherwise, continue to find next CTM.
			if ctmEndPattern.MatchString(line) {
				if ctm == "" {
					find = findCTM
				} else {
					break
				}
			} else {
				// Add content to the CTM.
				content := ctmContentPattern.FindString(line)
				if content != "" {
					ctm += content
				}
			}
		}
	}
	return ctm, nil
}
