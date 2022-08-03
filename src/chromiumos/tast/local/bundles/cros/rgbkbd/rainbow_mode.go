// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package rgbkbd

import (
	"context"
	"io/ioutil"
	"log"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/rgbkbd"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         RainbowMode,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that rainbow mode updates the correct number of keys for each device",
		Contacts: []string{
			"michaelcheco@google.com",
			"cros-peripherals@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

// RainbowMode constants defined in: src/platform2/rgbkbd/constants.h.
// RgbKeyboardCapabilities enum defined in: src/platform2/system_api/dbus/rgbkbd/dbus-constants.h.

// RainbowMode checks that rainbow mode updates the correct number of keys for each device.
func RainbowMode(ctx context.Context, s *testing.State) {
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
			name:           "kIndividualKey",
			capability:     individualKey,
			expectedLogLen: 77,
		}, {
			name:           "kFourZoneFortyLed",
			capability:     fourZoneFortyLed,
			expectedLogLen: 40,
		},
		{
			name:           "kFourZoneTwelveLed",
			capability:     fourZoneTwelveLed,
			expectedLogLen: 12,
		},
		{
			name:           "kFourZoneFourLed",
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
			err = rgbkbdService.SetRainbowMode(ctx)
			if err != nil {
				s.Fatal("Failed to call set rainbow mode: ", err)
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
				s.Fatalf("Unexpected # of calls to SetKeyColor, Expected (%d) | Actual (%d)", tc.expectedLogLen, count)
			}
		})
	}
}
