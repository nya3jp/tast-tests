// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package familylink

import (
	"context"
	"fmt"
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
		Func:         BedTimeLimit,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify the bed time limit works correctly for Family Link account",
		Contacts:     []string{"xiqiruan@chromium.org", "cros-families-eng+test@google.com", "chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
		VarDeps:      []string{"family.unicornEmail", "family.unicornPassword"},
		Fixture:      "familyLinkUnicornPolicyLogin",
	})
}

func BedTimeLimit(ctx context.Context, s *testing.State) {
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
	bedTimeDuration := time.Minute
	timeBeforeLocked := 30 * time.Second
	startAt := now.Add(timeBeforeLocked)
	endAt := startAt.Add(bedTimeDuration)

	usageLimitPolicy := familylink.CreateUsageTimeLimitPolicy()
	bedTimeLimitEntry := &policy.UsageTimeLimitValueTimeWindowLimitEntries{
		EffectiveDay:      strings.ToUpper(startAt.Weekday().String()),
		LastUpdatedMillis: strconv.FormatInt(now.Unix(), 10 /*base*/),
		EndsAt: &policy.RefTime{
			Hour:   endAt.Local().Hour(),
			Minute: endAt.Local().Minute(),
		},
		StartsAt: &policy.RefTime{
			Hour:   startAt.Local().Hour(),
			Minute: startAt.Local().Minute(),
		},
	}

	usageLimitPolicy.Val.TimeWindowLimit.Entries =
		append(usageLimitPolicy.Val.TimeWindowLimit.Entries, bedTimeLimitEntry)

	// Bed time duration is set by parent, which could be several hours starts at night
	// and ends in the morning. This test locks the screen after 30 seconds and unlocks it
	// after another minute. Family Link users have restrictions to prevent manipulating
	// the system clock.
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
	s.Log("Waiting for bed time starts in ", timeBeforeLocked)
	if _, err := lockscreen.WaitState(ctx, tconn,
		func(st lockscreen.State) bool { return st.Locked }, timeBeforeLocked+authTimeOut); err != nil {
		s.Fatal("Waiting for screen to be locked failed: ", err)
	}
	if err := ui.WaitUntilExists(nodewith.Name("Time for bed").Role(role.StaticText))(ctx); err != nil {
		s.Fatal("Time for bed message is missing: ", err)
	}

	s.Log("Waiting for bed time ends in ", bedTimeDuration)
	childUser := strings.ToLower(s.RequiredVar("family.unicornEmail"))
	if err := lockscreen.WaitForPasswordField(ctx, tconn, childUser, bedTimeDuration+authTimeOut); err != nil {
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
		s.Fatal(fmt.Sprintf("Waiting for screen to be unlocked failed (last status %+v): ", st), err)
	}

}
