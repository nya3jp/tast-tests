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
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ShareFilesToArc,
		Desc:     "A test to verify arc++ can save files to Downloads",
		Contacts: []string{"rnanjappan@chromium.org", "cros-arc-te@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p", "chrome"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm", "chrome"},
		}},
		Timeout: chrome.LoginTimeout + arc.BootTimeout + 120*time.Second,
		Vars:    []string{"arc.username", "arc.password"},
	})
}

func ShareFilesToArc(ctx context.Context, s *testing.State) {
	const (
		filename               = "file_example_JPG_500kB.jpg"
		crosPath               = "/home/chronos/user/Downloads/" + filename
		pkgName                = "com.google.android.apps.photos"
		allowButtonText        = "ALLOW"
		confirmButtonText      = "Confirm"
		DownloadButtonTxt      = "Download"
		skipButtonID           = "com.google.android.apps.photos:id/welcomescreens_skip_button"
		overflowButtonID       = "com.google.android.apps.photos:id/photos_overflow_icon"
		turnOnBackUpText       = "Turn on Backup"
		photoClassName         = "android.view.ViewGroup"
		AndroidButtonClassName = "android.widget.Button"
		AndroidTextClassName   = "android.widget.TextView"
		AndroidImageClassName  = "android.widget.ImageView"
		DefaultUITimeout       = 20 * time.Second
	)

	username := s.RequiredVar("arc.username")
	password := s.RequiredVar("arc.password")

	// Setup Chrome.
	cr, err := chrome.New(ctx, chrome.GAIALogin(), chrome.Auth(username, password, "gaia-id"), chrome.ARCSupported(),
		chrome.ExtraArgs(arc.DisableSyncFlags()...))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	// Optin to Play Store.
	s.Log("Opting into Play Store")
	if err := optin.Perform(ctx, cr, tconn); err != nil {
		s.Fatal("Failed to optin to Play Store: ", err)
	}
	if err := optin.WaitForPlayStoreShown(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for Play Store: ", err)
	}

	// Setup ARC.
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(ctx)

	// Install app.
	s.Log("Installing app")
	if err := playstore.InstallApp(ctx, a, d, pkgName, 3); err != nil {
		s.Fatal("Failed to install app: ", err)
	}

	if err := launcher.LaunchAndWaitForAppOpen(tconn, apps.Photos)(ctx); err != nil {
		s.Fatal("Failed to launch: ", err)
	}

	allowButton := d.Object(ui.ClassName(AndroidButtonClassName), ui.Text(allowButtonText))
	if err := allowButton.WaitForExists(ctx, DefaultUITimeout); err != nil {
		s.Log("Allow Button doesn't exist: ", err)
	} else if err := allowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on allowButton: ", err)
	}

	turnOnBackUpButton := d.Object(ui.ClassName(AndroidButtonClassName), ui.Text(turnOnBackUpText))
	if err := turnOnBackUpButton.WaitForExists(ctx, DefaultUITimeout); err != nil {
		s.Log("TurnOn BackUp Button doesn't exist: ", err)
	} else if err := turnOnBackUpButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on turnOnBackUpButton: ", err)
	}

	skipButton := d.Object(ui.ID(skipButtonID))
	if err := skipButton.WaitForExists(ctx, DefaultUITimeout); err != nil {
		s.Log("Skip Button doesn't exist: ", err)
	} else if err := skipButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on skipButton: ", err)
	}

	confirmButton := d.Object(ui.ClassName(AndroidButtonClassName), ui.Text(confirmButtonText))
	if err := confirmButton.WaitForExists(ctx, DefaultUITimeout); err != nil {
		s.Log("Confirm Button doesn't exist: ", err)
	} else if err := confirmButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on confirmButton: ", err)
	}

	skipButton = d.Object(ui.ID(skipButtonID))
	if err := skipButton.WaitForExists(ctx, DefaultUITimeout); err != nil {
		s.Log("Skip Button doesn't exist: ", err)
	} else if err := skipButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on skipButton: ", err)
	}

	selectPhoto := d.Object(ui.ClassName(photoClassName), ui.Index(2))
	if err := selectPhoto.WaitForExists(ctx, DefaultUITimeout); err != nil {
		s.Log("selectPhoto doesn't exist at Index 2: ", err)
		selectPhoto = d.Object(ui.ClassName(photoClassName), ui.Index(1))
		if err = selectPhoto.WaitForExists(ctx, 0); err != nil {
			s.Fatal("selectPhoto doesn't exist at Index 1 : ", err)
		}
	}
	if err := selectPhoto.Click(ctx); err != nil {
		s.Fatal("Failed to click on selectPhoto : ", err)
	}

	overflowButton := d.Object(ui.ClassName(AndroidImageClassName), ui.ID(overflowButtonID))
	if err := overflowButton.WaitForExists(ctx, DefaultUITimeout); err != nil {
		s.Fatal("Overflow Button doesn't exist : ", err)
	}

	if err := overflowButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on overflowButton : ", err)
	}

	downloadButton := d.Object(ui.ClassName(AndroidTextClassName), ui.Text(DownloadButtonTxt))
	if err := downloadButton.WaitForExists(ctx, DefaultUITimeout); err != nil {
		s.Fatal("Download Button doesn't exist : ", err)
	}
	if err := downloadButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on Downloadbutton : ", err)
	}

	if err := testing.Sleep(ctx, 30*time.Second); err != nil {
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
