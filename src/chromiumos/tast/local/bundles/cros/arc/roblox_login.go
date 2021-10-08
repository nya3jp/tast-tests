// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
	"context"
	"strconv"
	"strings"
	"time"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         RobloxLogin,
		Desc:         "Install Roblox from the Play Store, and log in",
		Contacts:     []string{"cwd@google.com"},
		SoftwareDeps: []string{"arc", "chrome"},
		Timeout:      5 * time.Hour,
		VarDeps:      []string{"ui.gaiaPoolDefault"},
		Vars:         []string{"wiggle"},
	})
}

func robloxGameIsRunning(infos []arc.TaskInfo) bool {
	for _, tinfo := range infos {
		acount := len(tinfo.ActivityInfos)
		if acount > 0 {
			ainfo := tinfo.ActivityInfos[acount-1]
			if ainfo.ActivityName == ".game.ActivityGame" && ainfo.PackageName == "com.roblox.client" {
				return true
			}
		}
	}
	return false
}

func RobloxLogin(ctx context.Context, s *testing.State) {
	const (
		pkgName      = "com.roblox.client"
		activityName = "com.roblox.client.startup.ActivitySplash"
	)

	// Setup Chrome.
	cr, err := chrome.New(ctx,
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
		chrome.ARCSupported(),
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

	if err := optin.WaitForPlayStoreShown(ctx, tconn, time.Minute); err != nil {
		s.Fatal("Failed to wait for Play Store: ", err)
	}

	// Setup ARC.
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close(ctx)

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(ctx)

	// Install app.
	s.Log("Installing app")
	if err := playstore.InstallApp(ctx, a, d, pkgName, -1); err != nil {
		s.Fatal("Failed to install app: ", err)
	}

	// Start Roblox.
	roblox, err := arc.NewActivity(a, pkgName, activityName)
	if err != nil {
		s.Fatal("Failed to start Roblox: ", err)
	}
	//defer roblox.Close()

	roblox.Start(ctx, tconn)
	s.Log("Started Roblox")

	if err := a.WaitForLogcat(ctx, func(line string) bool {
		return strings.Contains(line, "ASMA.onAppReady() Landing")
	}); err != nil {
		s.Fatal("Failed to wait for Roblox landing in logcat")
	}
	s.Log("Roblox landing screen is up")

	size, err := roblox.DisplaySize(ctx)
	if err != nil {
		s.Fatal("Failed to get display size of Roblox: ", err)
	}
	if err := d.Click(ctx, (size.Width*683)/1366, (size.Height*450)/768); err != nil {
		s.Fatal("Failed to click on login button: ", err)
	}
	s.Log("Clicked on login button")

	if err := a.WaitForLogcat(ctx, func(line string) bool {
		return strings.Contains(line, "ASMA.onAppReady() Login")
	}); err != nil {
		s.Fatal("Failed to wait for Roblox login in logcat")
	}
	s.Log("Login screen is up")

	// Wiggle wiggle
	if val, ok := s.Var("wiggle"); ok {
		s.Log("Wiggle is set, load a game and wiggle will start")
		if err := a.WaitForLogcat(ctx, func(line string) bool {
			return strings.Contains(line, "SessionReporterState_GameLoaded")
		}); err != nil {
			s.Fatal("Failed to wait for Roblox game load in logcat")
		}
		s.Log("Roblox game loaded")

		if wiggle, err := strconv.ParseBool(val); err != nil {
			s.Fatal("Failed to parse wiggle Var: ", err)
		} else if wiggle {
			for {
				if infos, err := a.TaskInfosFromDumpsys(ctx); err != nil {
					s.Fatal("Failed to get TaskInfos to check if Roblox is running")
				} else if !robloxGameIsRunning(infos) {
					s.Fatal("Roblox game is not running")
				}

				// Drag left.
				if err := d.Drag(ctx, (size.Width*683)/1366, (size.Height*384)/1366, (size.Width*342)/1366, (size.Height*384)/1366, 100); err != nil {
					s.Fatal("Failed to drag for wiggle: ", err)
				}
				if err := testing.Sleep(ctx, time.Second); err != nil {
					s.Fatal("Failed to sleep between left and right wiggle: ", err)
				}
				// Drag right.
				if err := d.Drag(ctx, (size.Width*683)/1366, (size.Height*384)/1366, (size.Width*1024)/1366, (size.Height*384)/1366, 100); err != nil {
					s.Fatal("Failed to drag for wiggle: ", err)
				}
				s.Log("Wiggle")
				if err := testing.Sleep(ctx, time.Minute); err != nil {
					s.Fatal("Failed to sleep after wiggle: ", err)
				}
			}
		}
	}

}
