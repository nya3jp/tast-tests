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

	// Enable and Disable Capslock.
	kb.Accel(ctx, "alt+search")
	kb.Accel(ctx, "alt+search")

	content, err := ioutil.ReadFile("/run/rgbkbd/log")
	if err != nil {
		log.Fatal(err)
	}

	var actualContent = string(content)
	res, errMsg := rgbkbd.VerifyInitialLogState(actualContent)
	remainingLog := rgbkbd.GetRemainingLog(actualContent)
	if !res {
		s.Fatalf("Logs do not match, err: %s", errMsg)
	}

	expectedLogLines := []string{"RGB::SetKeyColor - 44,255,255,210",
		"RGB::SetKeyColor - 57,255,255,210",
		"RGB::SetKeyColor - 44,0,81,231",
		"RGB::SetKeyColor - 57,0,81,231"}

	res, errMsg = rgbkbd.AreLogsEqual(expectedLogLines, remainingLog)

	if !res {
		s.Fatalf("Logs do not match, err: %s", errMsg)
	}

}
