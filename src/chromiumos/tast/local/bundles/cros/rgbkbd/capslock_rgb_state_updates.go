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
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/rgbkbd"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CapslockRgbStateUpdates,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that toggling Capslock updates the RGB backlight",
		Contacts: []string{
			"jimmyxgong@chromium.org",
			"cros-peripherals@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

// CapslockRgbStateUpdates verifies that enabling or disabling Capslock updates
// the RGB backlight.
func CapslockRgbStateUpdates(ctx context.Context, s *testing.State) {
	const (
		dbusName             = "org.chromium.Rgbkbd"
		dbusPath             = "/org/chromium/Rgbkbd"
		dbusInterface        = "org.chromium.Rgbkbd"
		individualKey uint32 = 1
		job                  = "rgbkbd"
	)

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

	err = rgbkbdService.SetTestingMode(ctx, individualKey)
	if err != nil {
		s.Error("Failed to set testing mode: ", err)
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

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	// Reset to default background color. Overrides wallpaper extracted
	// color which varies depending on the device used.
	err = rgbkbdService.SetStaticBackgroundColor(ctx, 255, 255, 210)

	// Enable and Disable Capslock.
	kb.Accel(ctx, "alt+search")
	kb.Accel(ctx, "alt+search")

	// TODO(michaelcheco): Add tast test that verifies calls made at startup
	// (initial caps lock state, default keyboard backlight color) when RGB keyboard
	// is supported.
	content, err := ioutil.ReadFile("/run/rgbkbd/log")
	if err != nil {
		log.Fatal(err)
	}

	expectedLogLines := []string{"RGB::SetKeyColor - 44,25,55,210",
		"RGB::SetKeyColor - 57,25,55,210",
		"RGB::SetKeyColor - 44,255,255,210",
		"RGB::SetKeyColor - 57,255,255,210"}

	logsMatch, msg := rgbkbd.AreLastLogLinesEqual(expectedLogLines, string(content))
	if !logsMatch {
		s.Fatalf("Logs do not match, err: %s", msg)
	}

}
