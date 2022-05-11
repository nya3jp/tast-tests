// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

const (
	playstorePackageName    = "com.android.vending"
	timeoutWaitForPlayStore = 5 * time.Minute
	loginPoolVar            = "arc.managedAccountPool"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         OptinManaged,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "A functional test that verifies OptIn flow for managed user",
		Contacts: []string{
			"arc-commercial@google.com",
			"mhasank@chromium.org",
			"yaohuali@google.com",
		},
		Attr: []string{"group:mainline", "group:arc-functional"},
		VarDeps: []string{
			loginPoolVar,
		},
		SoftwareDeps: []string{
			"chrome",
			"chrome_internal",
			"play_store",
		},
		Params: []testing.Param{{
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Timeout: 6 * time.Minute,
	})
}

func OptinManaged(ctx context.Context, s *testing.State) {
	// Actual username and password are read from vars/arc.yaml.
	creds, err := chrome.PickRandomCreds(s.RequiredVar(loginPoolVar))
	if err != nil {
		s.Fatal("Failed to get login creds: ", err)
	}

	// The policy means to force ARC to be enabled.
	policies := []policy.Policy{&policy.ArcEnabled{Val: true, Stat: policy.StatusSet}}

	fdms, err := arc.SetupFakePolicyServer(ctx, s.OutDir(), creds.User, policies)
	if err != nil {
		s.Fatal("Failed to setup fake policy server: ", err)
	}
	defer fdms.Stop(ctx)

	gaiaLogin := chrome.GAIALogin(creds)
	cr, err := arc.SetupManagedChrome(ctx, gaiaLogin, fdms)
	if err != nil {
		s.Fatal("Failed to setup chrome: ", err)
	}
	defer cr.Close(ctx)

	s.Log("Performing optin")

	// Verify that ARC opt-in is successful, by trying to open Play Store and checking that its window is shown.
	conn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API Conn: ", err)
	}
	if err := optin.LaunchAndWaitForPlayStore(ctx, conn, cr, timeoutWaitForPlayStore); err != nil {
		s.Fatal("Optin failed: ", err)
	}
}
