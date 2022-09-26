// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/pci"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/lockscreen"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AllowScreenLock,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Behavior of AllowScreenLock policy, checking whether the screen can be locked after setting the policy",
		Contacts: []string{
			"gabormagda@google.com", // Test author
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      fixture.ChromePolicyLoggedIn,
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.AllowScreenLock{}, pci.VerifiedFunctionalityUI),
		},
	})
}

// AllowScreenLock tests the AllowScreenLock policy.
func AllowScreenLock(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Open a keyboard device.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open keyboard device: ", err)
	}
	defer kb.Close()

	for _, param := range []struct {
		name       string
		wantLocked bool                    // wantLocked is the wanted lock screen locked state after trying to lock the screen.
		value      *policy.AllowScreenLock // value is the value of the policy.
	}{
		{
			name:       "allow",
			wantLocked: true,
			value:      &policy.AllowScreenLock{Val: true},
		},
		{
			name:       "deny",
			wantLocked: false,
			value:      &policy.AllowScreenLock{Val: false},
		},
		{
			name:       "unset",
			wantLocked: true,
			value:      &policy.AllowScreenLock{Stat: policy.StatusUnset},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Connect to Test API.
			tconn, err := cr.TestAPIConn(ctx)
			if err != nil {
				s.Fatal("Failed to connect to test API: ", err)
			}

			// Try to lock the screen with keyboard shortcut.
			s.Log("Trying to lock the screen")
			if err := kb.Accel(ctx, "Search+L"); err != nil {
				s.Fatal("Failed to write events: ", err)
			}
			ui := uiauto.New(tconn)
			lockScreenSubmitNode := nodewith.Name("Submit").ClassName("ArrowButtonView")
			if param.wantLocked {
				if err := ui.WaitUntilExists(lockScreenSubmitNode)(ctx); err != nil {
					s.Error("Failed to find the lock screen submit button: ", err)
				}
			} else {
				if err := ui.EnsureGoneFor(lockScreenSubmitNode, 15*time.Second)(ctx); err != nil {
					s.Error("Lock screen appeared but it shouldn't: ", err)
				}
			}

			// Check if the logout button is shown.
			// If the lock screen is disabled, the button is not there.
			// If enabled, the screen is locked with the hotkey, but the system tray can be
			// opened on the lock screen as well, but the button should not be there as the
			// screen is already locked.
			if err := quicksettings.Show(ctx, tconn); err != nil {
				s.Fatal("Failed to open the system tray: ", err)
			}

			if err := uiauto.Combine("Check lock screen from system tray",
				ui.WaitUntilExists(nodewith.Name("Shut down").ClassName("IconButton")),
				ui.WaitUntilGone(nodewith.Name("Lock").ClassName("IconButton")),
			)(ctx); err != nil {
				s.Error("Failed to check the lock screen button: ", err)
			}

			// Wait until the lock state is the wanted value or until timeout.
			state, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return st.Locked == param.wantLocked }, 10*time.Second)
			if err != nil {
				s.Errorf("Failed to wait for screen lock state %t: %v  (last status %+v)", param.wantLocked, err, state)
			}

			// Unlock the screen if needed.
			if state.Locked {
				s.Log("Unlocking the screen with password")
				if st, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return st.ReadyForPassword }, 10*time.Second); err != nil {
					s.Fatalf("Failed to wait until lock screen is ready for password: %v (last status %+v)", err, st)
				}
				if err := lockscreen.EnterPassword(ctx, tconn, fixtures.Username, fixtures.Password, kb); err != nil {
					s.Fatal("Failed to unlock the screen: ", err)
				}
				if st, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return !st.Locked }, 30*time.Second); err != nil {
					s.Errorf("Failed to wait for screen to be unlocked: %v (last status %+v)", err, st)
				}
			}
		})
	}
}
