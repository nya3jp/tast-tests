// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	pmpb "chromiumos/system_api/power_manager_proto"
	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/pci"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/checked"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/lockscreen"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/restriction"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromeOsLockOnIdleSuspend,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Behavior of ChromeOsLockOnIdleSuspend policy, checking the correspoding toggle button states (restriction and checked) after setting the policy and checking the appearance of the lock screen after the lid is closed",
		Contacts: []string{
			"gabormagda@google.com", // Test author
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      fixture.ChromePolicyLoggedIn,
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.ChromeOsLockOnIdleSuspend{}, pci.VerifiedFunctionalityUI),
		},
	})
}

// ChromeOsLockOnIdleSuspend tests the ChromeOsLockOnIdleSuspend policy.
func ChromeOsLockOnIdleSuspend(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	emitter, err := power.NewPowerManagerEmitter(ctx)
	if err != nil {
		s.Fatal("Unable to create power manager emitter: ", err)
	}
	defer func(cleanupCtx context.Context) {
		if err := emitter.Stop(cleanupCtx); err != nil {
			s.Log("Unable to stop emitter: ", err)
		}
	}(ctx)

	const lockTimeoutSeconds = 5

	for _, param := range []struct {
		name                        string
		wantRestriction             restriction.Restriction           // wantRestriction is the wanted restriction state of the checkboxes in Browsing history.
		wantChecked                 checked.Checked                   // wantChecked is the wanted checked state of the checkboxes in Browsing history.
		crosLockOnIdleSuspendPolicy *policy.ChromeOsLockOnIdleSuspend // crosLockOnIdleSuspendPolicy is the value of the ChromeOsLockOnIdleSuspend policy.
	}{
		{
			name:                        "forced",
			wantRestriction:             restriction.Disabled,
			wantChecked:                 checked.True,
			crosLockOnIdleSuspendPolicy: &policy.ChromeOsLockOnIdleSuspend{Val: true},
		},
		{
			name:                        "disabled",
			wantRestriction:             restriction.Disabled,
			wantChecked:                 checked.False,
			crosLockOnIdleSuspendPolicy: &policy.ChromeOsLockOnIdleSuspend{Val: false},
		},
		{
			name:                        "unset",
			wantRestriction:             restriction.None,
			wantChecked:                 checked.False,
			crosLockOnIdleSuspendPolicy: &policy.ChromeOsLockOnIdleSuspend{Stat: policy.StatusUnset},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.crosLockOnIdleSuspendPolicy}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Open the Security and sign-in page where the affected toggle button can be found.
			if err := policyutil.OSSettingsPageWithPassword(ctx, cr, "osPrivacy/lockScreen", fixtures.Password).
				SelectNode(ctx, nodewith.
					Role(role.ToggleButton).
					Name("Lock when sleeping or lid is closed")).
				Restriction(param.wantRestriction).
				Checked(param.wantChecked).
				Verify(); err != nil {
				s.Error("Unexpected OS settings state: ", err)
			}

			// Close the lid and check if the policy works correctly.
			// TODO: chromium:1264764 check if the policy works correctly when the user is idle and the device suspends.
			var shouldLockDevice bool = param.crosLockOnIdleSuspendPolicy.Val

			// LidCloseAction is "Suspend" by default.
			if param.crosLockOnIdleSuspendPolicy.Stat == policy.StatusUnset {
				shouldLockDevice = false
			}

			eventType := pmpb.InputEvent_LID_CLOSED
			if err := emitter.EmitInputEvent(ctx, &pmpb.InputEvent{Type: &eventType}); err != nil {
				s.Fatal("Send LID_CLOSED failed: ", err)
			}

			st, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return st.Locked && st.ReadyForPassword }, lockTimeoutSeconds*time.Second)
			screenLocked := st.Locked && st.ReadyForPassword

			if screenLocked && !shouldLockDevice {
				s.Fatal("Screen should not be locked: ", err)
			}

			if !screenLocked && shouldLockDevice {
				s.Fatal("Screen should be locked: ", err)
			}

			eventType = pmpb.InputEvent_LID_OPEN
			if err := emitter.EmitInputEvent(ctx, &pmpb.InputEvent{Type: &eventType}); err != nil {
				s.Fatal("Send LID_OPEN failed: ", err)
			}

			unlockScreen(ctx, fixtures.Password, s, tconn)
		})
	}
}

// unlockScreen unlocks the screen by typing the given password.
func unlockScreen(ctx context.Context, password string, s *testing.State, tconn *chrome.TestConn) {
	const authTimeoutSeconds = 5

	s.Log("Unlocking screen by typing the given password")

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed creating keyboard: ", err)
	}
	defer kb.Close()

	if err := kb.Type(ctx, password+"\n"); err != nil {
		s.Fatal("Typing the password failed: ", err)
	}
	if st, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return !st.Locked }, authTimeoutSeconds*time.Second); err != nil {
		s.Fatalf("Waiting for screen to be unlocked failed: %v (last status %+v)", err, st)
	}
}
