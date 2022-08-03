// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package personalization

import (
	"context"
	"io/ioutil"
	"log"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/personalization"
	"chromiumos/tast/local/rgbkbd"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SelectRainbowMode,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that rainbow mode updates the correct number of keys for each device",
		Contacts: []string{
			"michaelcheco@google.com",
			"cros-peripherals@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      3 * time.Minute,
	})
}

// RainbowMode constants defined in: src/platform2/rgbkbd/constants.h.
// RgbKeyboardCapabilities enum defined in: src/platform2/system_api/dbus/rgbkbd/dbus-constants.h.

// SelectRainbowMode checks that selecting the "Rainbow" keyboard backlight color  updates the correct number of keys for each device.
func SelectRainbowMode(ctx context.Context, s *testing.State) {
	const (
		dbusName                 = "org.chromium.Rgbkbd"
		individualKey     uint32 = 1
		fourZoneFortyLed  uint32 = 2
		fourZoneTwelveLed uint32 = 3
		fourZoneFourLed   uint32 = 4
		job                      = "rgbkbd"
	)

	for _, tc := range []struct {
		name           string
		capability     uint32
		expectedLogLen int
	}{
		{
			name:           "Vell",
			capability:     individualKey,
			expectedLogLen: 77,
		}, {
			name:           "Taniks",
			capability:     fourZoneFortyLed,
			expectedLogLen: 40,
		},
		{
			name:           "Osiris",
			capability:     fourZoneTwelveLed,
			expectedLogLen: 12,
		},
		{
			name:           "Mithrax",
			capability:     fourZoneFourLed,
			expectedLogLen: 4,
		},
	} {
		s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
			cleanupCtx := ctx
			ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
			defer cancel()

			s.Logf("Restarting %s job and waiting for %s service", job, dbusName)
			if err := upstart.RestartJob(ctx, job); err != nil {
				s.Fatalf("Failed to start %s: %v", job, err)
			}

			rgbkbdService, err := rgbkbd.NewRgbkbd(ctx)
			if err != nil {
				s.Fatalf("Failed to connect to %s: %v", dbusName, err)
			}

			err = rgbkbdService.SetTestingMode(ctx, tc.capability)
			if err != nil {
				s.Fatal("Failed to set testing mode: ", err)
			}

			cr, err := chrome.New(ctx, chrome.EnableFeatures("RgbKeyboard"))
			if err != nil {
				s.Fatal("Failed to start Chrome: ", err)
			}
			defer cr.Close(cleanupCtx)

			tconn, err := cr.TestAPIConn(ctx)
			if err != nil {
				s.Fatal("Failed to connect Test API: ", err)
			}
			defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)
			// The test has a dependency of network speed, so we give uiauto.Context ample
			// time to wait for nodes to load.
			ui := uiauto.New(tconn).WithTimeout(30 * time.Second)

			if err := uiauto.Combine("open Personalization Hub and verify Keyboard settings available",
				personalization.OpenPersonalizationHub(ui),
				ui.WaitUntilExists(nodewith.Role(role.StaticText).NameContaining("Keyboard backlight")))(ctx); err != nil {
				s.Fatal("Failed to show Keyboard settings: ", err)
			}
			if err := selectRainbowMode(ui)(ctx); err != nil {
				s.Fatal("Failed to select backlight color rainbow: ", err)
			}

			content, err := ioutil.ReadFile("/run/rgbkbd/log")
			if err != nil {
				log.Fatal(err)
			}

			count, err := rgbkbd.RainbowModeCount(string(content))
			if err != nil {
				s.Fatal("Failed to get rainbow mode count: ", err)
			}
			if count != tc.expectedLogLen {
				s.Fatalf("Unexpected # of calls to SetKeyColor ... got %d, want %d", tc.expectedLogLen, count)
			}
		})
	}
}

func selectRainbowMode(ui *uiauto.Context) uiauto.Action {
	rainbowColor := "Rainbow"
	colorOption := nodewith.HasClass("color-container").Name(rainbowColor)
	selectedColor := nodewith.HasClass("color-container tast-selected-color").Name(rainbowColor)

	return uiauto.Combine("validate the selected backlight color",
		ui.MakeVisible(colorOption),
		ui.LeftClick(colorOption),
		ui.WaitUntilExists(selectedColor))
}
