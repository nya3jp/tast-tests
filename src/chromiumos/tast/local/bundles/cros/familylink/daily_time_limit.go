// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package familylink

import (
	"context"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/familylink"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/lockscreen"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DailyTimeLimit,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify the daily time limit works correctly for Family Link account",
		Contacts:     []string{"xiqiruan@chromium.org", "cros-families-eng+test@google.com", "chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
		VarDeps:      []string{"family.unicornEmail", "family.unicornPassword"},
		Fixture:      "familyLinkUnicornPolicyLogin",
	})
}

func DailyTimeLimit(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	tconn := s.FixtValue().(familylink.HasTestConn).TestConn()

	// Make sure screen is not locked.
	s.Log("Assert the screen is not locked")
	if _, err := lockscreen.WaitState(ctx, tconn,
		func(st lockscreen.State) bool { return !st.Locked }, 30*time.Second); err != nil {
		s.Fatal("Waiting for screen to be unlocked failed: ", err)
	}

	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	now := time.Now()
	usageLimitPolicy := familylink.CreateUsageTimeLimitPolicy()

	// Set daily limit to be 1m to shorten the test.
	dailyScreenTimeLimit := time.Minute
	resetInMin := dailyScreenTimeLimit + time.Minute
	// In real life, the reset time is 6:00 am. This test sets the reset time
	// to be 1m after daily limit ends and the screen is locked (which is
	// 2m after logged in) without changing the system clock. Family Link users
	// have restrictions to prevent manipulating the system clock.
	reset := now.Add(resetInMin)
	usageLimitPolicy.Val.TimeUsageLimit.ResetAt = &policy.RefTime{
		Hour:   reset.Local().Hour(),
		Minute: reset.Local().Minute(),
	}

	dailyLimitEntry := &policy.RefTimeUsageLimitEntry{
		LastUpdatedMillis: strconv.FormatInt(now.Unix(), 10 /*base*/),
		UsageQuotaMins:    int(dailyScreenTimeLimit.Minutes()),
	}

	// The daily limit is applied between reset time of day N and day N+1. For example,
	// if we set 1 minute daily limit on Tuesday with the reset time to be 17:00, the
	// 1 minute daily limit will be applied between 17:00 Tuesday to 16:59 Wednesday.
	// This line calculate which date in the week should be set limit on base on `reset`
	// and set this date with `dailyLimitEntry`.
	oneDayBackward := -time.Hour * 24
	switch weekday := reset.Add(oneDayBackward).Weekday(); weekday {
	case time.Sunday:
		usageLimitPolicy.Val.TimeUsageLimit.Sunday = dailyLimitEntry
	case time.Monday:
		usageLimitPolicy.Val.TimeUsageLimit.Monday = dailyLimitEntry
	case time.Tuesday:
		usageLimitPolicy.Val.TimeUsageLimit.Tuesday = dailyLimitEntry
	case time.Wednesday:
		usageLimitPolicy.Val.TimeUsageLimit.Wednesday = dailyLimitEntry
	case time.Thursday:
		usageLimitPolicy.Val.TimeUsageLimit.Thursday = dailyLimitEntry
	case time.Friday:
		usageLimitPolicy.Val.TimeUsageLimit.Friday = dailyLimitEntry
	case time.Saturday:
		usageLimitPolicy.Val.TimeUsageLimit.Saturday = dailyLimitEntry
	}

	policies := []policy.Policy{
		usageLimitPolicy,
	}
	pb := policy.NewBlob()
	pb.PolicyUser = s.FixtValue().(familylink.HasPolicyUser).PolicyUser()
	pb.AddPolicies(policies)

	if err := policyutil.ServeBlobAndRefresh(ctx, fdms, cr, pb); err != nil {
		s.Fatal("Failed to serve policies: ", err)
	}

	s.Log("Verifying policies were delivered to device")
	if err := policyutil.Verify(ctx, tconn, policies); err != nil {
		s.Fatal("Failed to verify policies: ", err)
	}

	ui := uiauto.New(tconn)
	authTimeOut := 10 * time.Second

	// The tested account on DUT might have been active before this test. The screen might be
	// locked less than `dailyScreenTimeLimit`.
	s.Log("Waiting for daily limit reaches at most in ", dailyScreenTimeLimit)
	if _, err := lockscreen.WaitState(ctx, tconn,
		func(st lockscreen.State) bool { return st.Locked }, dailyScreenTimeLimit+authTimeOut); err != nil {
		s.Fatal("Waiting for screen to be locked failed: ", err)
	}
	if err := ui.WaitUntilExists(nodewith.Name("Time is up").Role(role.StaticText))(ctx); err != nil {
		s.Fatal("Time is up message is missing: ", err)
	}

	s.Log("Waiting for daily limit reset at most in ", resetInMin)
	childUser := strings.ToLower(s.RequiredVar("family.unicornEmail"))
	if err := lockscreen.WaitForPasswordField(ctx, tconn, childUser, resetInMin+authTimeOut); err != nil {
		s.Error("Password text field did not appear in the UI: ", err)
	}

	s.Log("Trying to unlock screen")
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()
	if err := lockscreen.EnterPassword(ctx, tconn, childUser,
		s.RequiredVar("family.unicornPassword"), kb); err != nil {
		s.Fatal("Entering password failed: ", err)
	}
	if st, err := lockscreen.WaitState(ctx, tconn,
		func(st lockscreen.State) bool { return st.LoggedIn }, authTimeOut); err != nil {
		s.Fatalf("Waiting for screen to be unlocked failed (last status %+v): %v", st, err)
	}

}
