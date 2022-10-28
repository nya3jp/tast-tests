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
	"chromiumos/tast/ctxutil"
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
		Desc:         "Behavior of ChromeOsLockOnIdleSuspend policy, checking the correspoding toggle button states (restriction and checked) and the lock screen after the lid is closed",
		Contacts: []string{
			"gabormagda@google.com", // Test author
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      fixture.ChromePolicyLoggedIn,
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.ChromeOsLockOnIdleSuspend{}, pci.VerifiedFunctionalityUI),
			pci.SearchFlag(&policy.ChromeOsLockOnIdleSuspend{}, pci.VerifiedFunctionalityOS),
		},
	})
}

// ChromeOsLockOnIdleSuspend tests the ChromeOsLockOnIdleSuspend policy.
func ChromeOsLockOnIdleSuspend(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 20*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	emitter, err := power.NewPowerManagerEmitter(ctx)
	if err != nil {
		s.Fatal("Unable to create power manager emitter: ", err)
	}
	defer func(ctx context.Context) {
		if err := emitter.Stop(ctx); err != nil {
			s.Log("Unable to stop emitter: ", err)
		}
	}(cleanupCtx)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed creating keyboard: ", err)
	}
	defer kb.Close()

	const lockTimeout = 5 * time.Second

	for _, param := range []struct {
		name            string
		wantLockDevice  bool                              // wantLockDevice is the wanted lock state of the device after the lid is closed.
		wantRestriction restriction.Restriction           // wantRestriction is the wanted restriction state of the checkboxes in Browsing history.
		wantChecked     checked.Checked                   // wantChecked is the wanted checked state of the checkboxes in Browsing history.
		policyValue     *policy.ChromeOsLockOnIdleSuspend // policyValue is the value of the ChromeOsLockOnIdleSuspend policy.
	}{
		{
			name:            "forced",
			wantLockDevice:  true,
			wantRestriction: restriction.Disabled,
			wantChecked:     checked.True,
			policyValue:     &policy.ChromeOsLockOnIdleSuspend{Val: true},
		},
		{
			name:            "disabled",
			wantLockDevice:  false,
			wantRestriction: restriction.Disabled,
			wantChecked:     checked.False,
			policyValue:     &policy.ChromeOsLockOnIdleSuspend{Val: false},
		},
		{
			name:            "unset",
			wantLockDevice:  false,
			wantRestriction: restriction.None,
			wantChecked:     checked.False,
			policyValue:     &policy.ChromeOsLockOnIdleSuspend{Stat: policy.StatusUnset},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.policyValue}); err != nil {
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
			// TODO(b/257211713): check if the policy works correctly when the user is idle and the device suspends.
			eventType := pmpb.InputEvent_LID_CLOSED
			if err := emitter.EmitInputEvent(ctx, &pmpb.InputEvent{Type: &eventType}); err != nil {
				s.Fatal("Send LID_CLOSED failed: ", err)
			}
			// Defer the screen unlock to ensure subsequent tests aren't affected by the screen remaining locked.
			defer func(ctx context.Context) {
				eventType := pmpb.InputEvent_LID_OPEN
				if err := emitter.EmitInputEvent(ctx, &pmpb.InputEvent{Type: &eventType}); err != nil {
					s.Fatal("Send LID_OPEN failed: ", err)
				}

				const authTimeout = 5 * time.Second

				s.Log("Unlocking screen by typing the given password")

				if err := kb.Type(ctx, fixtures.Password+"\n"); err != nil {
					s.Fatal("Typing the password failed: ", err)
				}
				if st, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return !st.Locked }, authTimeout); err != nil {
					s.Fatalf("Waiting for screen to be unlocked failed: %v (last status %+v)", err, st)
				}
			}(cleanupCtx)

			st, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return st.Locked && st.ReadyForPassword }, lockTimeout)
			screenLocked := st.Locked && st.ReadyForPassword

			if screenLocked && !param.wantLockDevice {
				s.Fatal("Screen should not be locked: ", err)
			}

			if !screenLocked && param.wantLockDevice {
				s.Fatal("Screen should be locked: ", err)
			}
		})
	}
}
