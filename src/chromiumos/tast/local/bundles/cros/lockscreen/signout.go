// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lockscreen

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash/ashproc"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/lockscreen"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/cpu"
	"chromiumos/tast/local/crash"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/procutil"
	"chromiumos/tast/testing"
)

type testParam struct {
	withShortcut bool
	checkCrashes bool
	bt           browser.Type
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Signout,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Test signout from the lock screen",
		Contacts:     []string{"rsorokin@google.com", "chromeos-sw-engprod@google.com", "cros-oac@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		VarDeps:      []string{"ui.signinProfileTestExtensionManifestKey"},
		Params: []testing.Param{{
			Val: testParam{false, false, browser.TypeAsh},
		}, {
			Name: "shortcut",
			Val:  testParam{true, false, browser.TypeAsh},
		}, {
			Name: "check_crashes",
			Val:  testParam{false, true, browser.TypeAsh},
		}, {
			Name:              "lacros",
			Val:               testParam{false, false, browser.TypeLacros},
			ExtraSoftwareDeps: []string{"lacros"},
		}, {
			Name:              "shortcut_lacros",
			Val:               testParam{true, false, browser.TypeLacros},
			ExtraSoftwareDeps: []string{"lacros"},
		}, {
			Name:              "check_crashes_lacros",
			Val:               testParam{false, true, browser.TypeLacros},
			ExtraSoftwareDeps: []string{"lacros"},
		}},
		Vars: []string{browserfixt.LacrosDeployedBinary},
	})
}

func Signout(ctx context.Context, s *testing.State) {
	signoutWithKeyboardShortcut := s.Param().(testParam).withShortcut
	checkCrashes := s.Param().(testParam).checkCrashes
	bt := s.Param().(testParam).bt

	// Reserve some time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Separate function for the first chrome run to isolate from the second run. For example so it does not generate UI tree two times on error.
	func() {
		cr, br, closeBrowser, err := browserfixt.SetUpWithNewChrome(ctx, bt, lacrosfixt.NewConfigFromState(s),
			chrome.ExtraArgs("--force-tablet-mode=clamshell", "--disable-virtual-keyboard"))
		if err != nil {
			s.Fatalf("Chrome login failed with %v browser: %v", bt, err)
		}
		defer cr.Close(cleanupCtx)
		defer closeBrowser(cleanupCtx)

		tconn, err := cr.TestAPIConn(ctx)
		if err != nil {
			s.Fatal("Getting test API connection failed: ", err)
		}

		_, err = br.NewConn(ctx, "chrome://settings")
		if err != nil {
			s.Fatal("Failed to open a tab: ", err)
		}
		_, err = br.NewConn(ctx, "chrome://version")
		if err != nil {
			s.Fatal("Failed to open a tab: ", err)
		}

		if err := lockscreen.Lock(ctx, tconn); err != nil {
			s.Fatal("Failed to lock the screen: ", err)
		}

		var oldCrashes []string
		if checkCrashes {
			oldCrashes, err = crash.GetCrashes(crash.DefaultDirs()...)
			if err != nil {
				s.Fatal("GetCrashes failed: ", err)
			}
		}

		defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

		oldProc, err := ashproc.Root()
		if err != nil {
			s.Fatal("Failed to get Chrome root PID: ", err)
		}

		// We wait here to give Chrome more chances to shutdown in 3 seconds and
		// won't get killed by the session manager.
		if err := cpu.WaitUntilIdle(ctx); err != nil {
			s.Fatal("Failed to wait for CPU to become idle: ", err)
		}

		if signoutWithKeyboardShortcut {
			kb, err := input.Keyboard(ctx)
			if err != nil {
				s.Fatal("Failed to get keyboard: ", err)
			}
			if err := kb.Accel(ctx, "Ctrl+Shift+Q"); err != nil {
				s.Fatal("Failed emulate shortcut 1st press: ", err)
			}
			if err := kb.Accel(ctx, "Ctrl+Shift+Q"); err != nil {
				s.Fatal("Failed emulate shortcut 2nd press: ", err)
			}
		} else {
			// Sign out using button.
			ui := uiauto.New(tconn)
			signOutButton := nodewith.Name("Sign out").Role(role.Button)
			buttonFound, err := ui.IsNodeFound(ctx, signOutButton)
			if !buttonFound {
				s.Fatal("Signout button was not found: ", err)
			}

			// We click multiple times because device might be in the suspended
			// state after cpu.WaitUntilIdle call. And some clicks might be ignored
			// by the button.
			// We ignore errors here because when we click on "Sign out" button
			// Chrome shuts down and the connection is closed. So we always get an
			// error.
			ui.LeftClickUntil(signOutButton, ui.Gone(signOutButton))(ctx)
		}

		// Wait for Chrome restart
		if err := procutil.WaitForTerminated(ctx, oldProc, 30*time.Second); err != nil {
			s.Fatal("Timeout waiting for Chrome to shutdown: ", err)
		}
		if _, err := ashproc.WaitForRoot(ctx, 30*time.Second); err != nil {
			s.Fatal("Timeout waiting for Chrome to start: ", err)
		}

		if checkCrashes {
			newCrashes, err := crash.GetCrashes(crash.DefaultDirs()...)
			if err != nil {
				s.Fatal("GetCrashes failed: ", err)
			}
			if len(oldCrashes) != len(newCrashes) {
				oldCrashesMap := make(map[string]bool)
				for _, oldCrash := range oldCrashes {
					oldCrashesMap[oldCrash] = true
				}

				for _, crash := range newCrashes {
					// Save only new crashes
					if oldCrashesMap[crash] {
						continue
					}
					out := filepath.Join(s.OutDir(), filepath.Base(crash))
					if err := fsutil.CopyFile(crash, out); err != nil {
						s.Logf("Failed to save %v: %v", crash, err)
					}
				}
				s.Fatal("Something crashed during the test")
			}
		}
	}()

	// Restart chrome for testing
	cr, err := chrome.New(ctx, chrome.NoLogin(), chrome.KeepState(), chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")))
	if err != nil {
		s.Fatal("Chrome restart for testing failed: ", err)
	}
	tconn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		s.Fatal("Getting signing test API connection failed: ", err)
	}

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	if _, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return st.ReadyForPassword }, 10*time.Second); err != nil {
		s.Fatal("Failed to wait for login screen: ", err)
	}
}
