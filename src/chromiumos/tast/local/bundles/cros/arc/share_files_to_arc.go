// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"io/ioutil"
	"os"
	"time"

	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ShareFilesToArc,
		Desc:     "A test to verify arc++ can save files to Downloads",
		Contacts: []string{"rnanjappan@chromium.org", "cros-arc-te@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		Fixture:  "arcBooted",
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p", "chrome"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm", "chrome"},
		}},
		Timeout: chrome.LoginTimeout + arc.BootTimeout + 120*time.Second,
	})
}

func ShareFilesToArc(ctx context.Context, s *testing.State) {
	const (
		apk                    = "ArcDownloadTest.apk"
		pkgName                = "org.chromium.arc.testapp.download"
		cls                    = ".MainActivity"
		AndroidButtonClassName = "android.widget.Button"
		AndroidTextClassName   = "android.widget.TextView"
		AndroidImageClassName  = "android.widget.ImageView"
		DefaultUITimeout       = 20 * time.Second
		filename               = "1.jpg"
		crosPath               = "/home/chronos/user/Downloads/" + filename
		allowButtonText        = "ALLOW"
		DownloadButtonTxt      = "DOWNLOAD"
	)

	a := s.FixtValue().(*arc.PreData).ARC
	if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
		s.Fatal("Failed to install app: ", err)
	}

	cr := s.FixtValue().(*arc.PreData).Chrome
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	act, err := arc.NewActivity(a, pkgName, cls)
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	s.Log("Starting app")
	if err = act.Start(ctx, tconn); err != nil {
		s.Fatal("Failed to start app: ", err)
	}
	defer act.Stop(ctx, tconn)

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(ctx)

	downloadButton := d.Object(ui.ClassName(AndroidButtonClassName), ui.Text(DownloadButtonTxt))
	if err := downloadButton.WaitForExists(ctx, DefaultUITimeout); err != nil {
		s.Fatal("Download Button doesn't exist : ", err)
	}
	if err := downloadButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on Downloadbutton : ", err)
	}

	allowButton := d.Object(ui.ClassName(AndroidButtonClassName), ui.Text(allowButtonText))
	if err := allowButton.WaitForExists(ctx, DefaultUITimeout); err != nil {
		s.Log("Allow Button doesn't exist: ", err)
	} else if err := allowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on allowButton: ", err)
	}

	if err := testing.Sleep(ctx, 10*time.Second); err != nil {
		s.Fatal("Failed to wait until save finished : ", err)
	}

	actual, err := ioutil.ReadFile(crosPath)
	if err != nil {
		s.Error("Android -> CrOS failed: ", err)
	} else if len(actual) == 0 {
		s.Error("The file size is 0")
	}
	if err = os.Remove(crosPath); err != nil {
		s.Fatal("Failed to remove a file: ", err)
	}
}
