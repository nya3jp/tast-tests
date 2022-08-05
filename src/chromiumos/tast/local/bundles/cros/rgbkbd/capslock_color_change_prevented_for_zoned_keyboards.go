// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package rgbkbd

import (
	"context"
	"io/ioutil"
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
		Func:         CapslockColorChangePreventedForZonedKeyboards,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies that the caps lock keys do not change colors when SetCapsLockState is called from a zoned keyboard",
		Contacts: []string{
			"michaelcheco@google.com",
			"cros-peripherals@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      3 * time.Minute,
	})
}

// CapslockColorChangePreventedForZonedKeyboards verifies that enabling or disabling Capslock
// updates the RGB backlight.
func CapslockColorChangePreventedForZonedKeyboards(ctx context.Context, s *testing.State) {
	const (
		dbusName                = "org.chromium.Rgbkbd"
		dbusPath                = "/org/chromium/Rgbkbd"
		dbusInterface           = "org.chromium.Rgbkbd"
		fourZoneFortyLed uint32 = 2
		job                     = "rgbkbd"
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

	err = rgbkbdService.SetTestingMode(ctx, fourZoneFortyLed)
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

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	initialContent, err := ioutil.ReadFile("/run/rgbkbd/log")
	if err != nil {
		s.Fatal("Failed to read initial rgbkbd log contents: ", err)
	}

	// Enable and Disable Capslock.
	if err := kb.Accel(ctx, "alt+search"); err != nil {
		s.Fatal("Failed to press alt+search to enable caps lock: ", err)
	}

	if err := kb.Accel(ctx, "alt+search"); err != nil {
		s.Fatal("Failed to press alt+search to disable caps lock: ", err)
	}

	contentAfterCapsLockKeyPress, err := ioutil.ReadFile("/run/rgbkbd/log")
	if err != nil {
		s.Fatal("Failed to read rgbkbd log contents: ", err)
	}

	if string(initialContent) != string(contentAfterCapsLockKeyPress) {
		s.Fatal("Caps lock change logs written for zoned keyboard")
	}
}
