// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package familylink

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/familylink"
	"chromiumos/tast/local/chrome/ui/lockscreen"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DailyTimeLimit,
		Desc:         "Verify the daily time limit works correctly for Family Link account",
		Contacts:     []string{"xiqiruan@chromium.org", "cros-families-eng+test@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      chrome.GAIALoginTimeout + 5*time.Minute,
		Vars:         []string{"unicorn.childUser", "unicorn.childPassword"},
		Fixture:      "familyLinkUnicornPolicyLogin",
	})
}

func DailyTimeLimit(ctx context.Context, s *testing.State) {
	fdms := s.FixtValue().(*familylink.FixtData).FakeDMS
	cr := s.FixtValue().(*familylink.FixtData).Chrome
	tconn := s.FixtValue().(*familylink.FixtData).TestConn
	now := time.Now()

	// Set daily limit to be 1m to shorten the test.
	dailyLimit := time.Minute
	// In real life, the reset time is 6:00 am. Set the reset time
	// to be 1m after daily limit ends and the screen is locked (which is
	// 2m after logged in) without changing the system clock. Family Link users
	// have restrictions to prevent manipulating the system clock.
	reset := now.Add(dailyLimit + time.Minute)
	timeUsageLimit := policy.UsageTimeLimitValueTimeUsageLimit{
		ResetAt: &policy.RefTime{
			Hour:   reset.Local().Hour(),
			Minute: reset.Local().Minute(),
		},
	}
	daiyLimit := policy.RefTimeUsageLimitEntry{
		LastUpdatedMillis: strconv.FormatInt(now.Unix(), 10),
		UsageQuotaMins:    int(dailyLimit.Minutes()),
	}

	// The daily limit is applied between reset time of day N and day N+1. For example,
	// if we set 1 minute daily limit on Tuesday with the reset time to be 17:00, the
	// 1 minute daily limit will be applied between 17:00 Tuesday to 16:59 Wednesday.
	// This line caculate which date in the week should be set limit on base on `now`
	// and set this date with `daiyLimit`.
	reflect.ValueOf(&timeUsageLimit).Elem().FieldByName(now.Add(-time.Hour * 24).Weekday().String()).Set(reflect.ValueOf(&daiyLimit))

	policies := []policy.Policy{
		&policy.UsageTimeLimit{
			Val: &policy.UsageTimeLimitValue{
				TimeUsageLimit: &timeUsageLimit,
			},
		},
	}
	pb := fakedms.NewPolicyBlob()
	pb.PolicyUser = s.FixtValue().(*familylink.FixtData).PolicyUser
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
	s.Log("Waiting for daily limit reaches in ", dailyLimit)
	if _, err := lockscreen.WaitState(ctx, tconn,
		func(st lockscreen.State) bool { return st.Locked }, dailyLimit+authTimeOut); err != nil {
		s.Fatal("Waiting for screen to be locked failed: ", err)
	}
	if err := ui.Exists(nodewith.Name("Time is up").Role(role.StaticText))(ctx); err != nil {
		s.Fatal("Time is up message is missing: ", err)
	}

	s.Log("Waiting for daily limit reset in ", time.Minute)
	childUser := strings.ToLower(s.RequiredVar("unicorn.childUser"))
	if err := lockscreen.WaitForPasswordField(ctx, tconn, childUser, time.Minute+authTimeOut); err != nil {
		s.Error("Password text field did not appear in the UI: ", err)
	}

	s.Log("Trying to unlock screen")
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()
	if err := lockscreen.EnterPassword(ctx, tconn, childUser,
		s.RequiredVar("unicorn.childPassword")+"\n", kb); err != nil {
		s.Fatal("Entering password failed: ", err)
	}
	if st, err := lockscreen.WaitState(ctx, tconn,
		func(st lockscreen.State) bool { return st.LoggedIn }, authTimeOut); err != nil {
		s.Fatal(fmt.Sprintf("Waiting for screen to be unlocked failed (last status %+v):", st), err)
	}

}
