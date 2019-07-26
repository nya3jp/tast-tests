// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ImeBlockingVK,
		Desc:         "Checks if IME blocking works on ARC in tablet mode",
		Contacts:     []string{"tetsui@chromium.org", "arc-framework@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Data:         []string{"ArcImeBlockingTest.apk"},
		Timeout:      4 * time.Minute,
	})
}

func waitUntilShownOrHidden(ctx context.Context, tconn *chrome.Conn, shown bool) error {
	return tconn.EvalPromise(ctx, fmt.Sprintf(`
new Promise((resolve, reject) => {
	chrome.automation.getDesktop(root => {
		const check = () => {
			try {
				const keyboard = root.find({ attributes: { role: 'keyboard' }});
				if ((keyboard && !keyboard.state.invisible) == %t) {
					resolve();
					return;
				}
			} catch (e) {
				console.log(e);
			}
			setTimeout(check, 10);
		}
		check();
	});
})
`, shown), nil)
}

func ImeBlockingVK(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.ExtraArgs("--force-tablet-mode=touch_view", "--enable-virtual-keyboard"))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()

	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close()

	const (
		apk = "ArcImeBlockingTest.apk"
		pkg = "org.chromium.arc.testapp.imeblocking"
		cls = "org.chromium.arc.testapp.imeblocking.MainActivity"
	)

	s.Log("Installing app")
	if err := a.Install(ctx, s.DataPath(apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	s.Log("Starting app")
	if err := a.Command(ctx, "am", "start", "-W", fmt.Sprintf("%s/%s", pkg, cls)).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed starting app: ", err)
	}

	const (
		fieldID  = "org.chromium.arc.testapp.imeblocking:id/text"
		buttonID = "org.chromium.arc.testapp.imeblocking:id/button"
	)
	s.Log("Setting up app's initial state")
	field := d.Object(ui.ID(fieldID))
	if err := field.WaitForExists(ctx, 30*time.Second); err != nil {
		s.Fatal("Failed to find field: ", err)
	}
	if err := field.Click(ctx); err != nil {
		s.Fatal("Failed to click field: ", err)
	}

	if err := waitUntilShownOrHidden(ctx, tconn, true); err != nil {
		s.Fatal("Failed to show the virtual keyboard: ", err)
	}

	s.Log("Opening a dialog")
	button := d.Object(ui.ID(buttonID))
	if err := button.Click(ctx); err != nil {
		s.Fatal("Failed to click button: ", err)
	}

	if err := d.Object(ui.Text("OK"), ui.PackageName(pkg)).WaitForExists(ctx, 30*time.Second); err != nil {
		s.Fatal("Failed to wait for a dialog: ", err)
	}

	if err := waitUntilShownOrHidden(ctx, tconn, false); err != nil {
		s.Fatal("Failed to hide the virtual keyboard: ", err)
	}
}
