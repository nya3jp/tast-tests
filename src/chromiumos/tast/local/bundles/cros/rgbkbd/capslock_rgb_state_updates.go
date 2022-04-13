// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package rgbkbd

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"time"

	"github.com/godbus/dbus"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:          CapslockRgbStateUpdates,
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
// TODO(jimmyxgong): This is a stub test, implement when logger api is
// available.
func CapslockRgbStateUpdates(ctx context.Context, s *testing.State) {
	const (
		dbusName      = "org.chromium.Rgbkbd"
		dbusPath      = "/org/chromium/Rgbkbd"
		dbusInterface = "org.chromium.Rgbkbd"
		dbusMethod    = "SetTestingMode"

		job = "rgbkbd"
	)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

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

	s.Logf("Restarting %s job and waiting for %s service", job, dbusName)
	if err := upstart.RestartJob(ctx, job); err != nil {
		s.Fatalf("Failed to start %s: %v", job, err)
	}
	_, obj, err := dbusutil.Connect(ctx, dbusName, dbus.ObjectPath(dbusPath))
	if err != nil {
		s.Fatalf("Failed to connect to %s: %v", dbusName, err)
	}

	s.Log("Setting testing mode")
	if err := obj.CallWithContext(ctx, dbusInterface+ "." +
		dbusMethod, 0, true); err != nil {
		// s.Error("Failed to set testing mode: ", err)
	} else {
		s.Logf("Set Testing Mode")
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	kb.Accel(ctx, "alt+search")

	content, err := ioutil.ReadFile("/tmp/rgbkbd_log")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("File contents: %s", content)
}
