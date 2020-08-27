// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/chrome/ui/lockscreen"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/pre"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: AllowScreenLock,
		Desc: "Behavior of AllowScreenLock policy, checking whether the screen can be locked after setting the policy",
		Contacts: []string{
			"gabormagda@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          pre.User,
	})
}

// AllowScreenLock tests the AllowScreenLock policy.
func AllowScreenLock(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*pre.PreData).Chrome
	fdms := s.PreValue().(*pre.PreData).FakeDMS

	// Open a keyboard device.
	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open keyboard device: ", err)
	}
	defer keyboard.Close()

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
			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndRefresh(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Connect to Test API.
			tconn, err := cr.TestAPIConn(ctx)
			if err != nil {
				s.Fatal("Failed to connect to test API: ", err)
			}

			// Try to lock the screen.
			s.Log("Trying to lock the screen")
			if err := keyboard.Accel(ctx, "Search+L"); err != nil {
				s.Fatal("Failed to write events: ", err)
			}

			// Wait until the lock state is the wanted value or until timeout.
			state, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return st.Locked == param.wantLocked }, 10*time.Second)
			if err != nil {
				s.Errorf("Failed to wait for screen lock state %t: %v", param.wantLocked, err)
			}

			// Unlock the screen if needed.
			if state.Locked {
				s.Log("Unlocking the screen with password")
				if err := keyboard.Type(ctx, pre.Password+"\n"); err != nil {
					s.Fatal("Failed to type password: ", err)
				} else if _, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return !st.Locked }, 10*time.Second); err != nil {
					s.Fatal("Failed to wait for the screen to be unlocked with password: ", err)
				}
			}
		})
	}
}
