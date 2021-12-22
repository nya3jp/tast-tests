// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

const (
	playstorePackageName = "com.android.vending"
)

type optinTestParam struct {
	username string
	password string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         OptinManaged,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "A functional test that verifies OptIn flow for managed user",
		Contacts: []string{
			"arc-core@google.com",
			"mhasank@chromium.org",
			"yaohuali@google.com",
		},
		Attr: []string{"group:mainline", "group:arc-functional"},
		VarDeps: []string{
			"arc.OptinManaged.username",
			"arc.OptinManaged.password"},
		SoftwareDeps: []string{
			"chrome",
			"chrome_internal",
			"play_store",
		},
		Params: []testing.Param{{
			Val: optinTestParam{
				username: "arc.OptinManaged.username",
				password: "arc.OptinManaged.password",
			},
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name: "vm",
			Val: optinTestParam{
				username: "arc.OptinManaged.username",
				password: "arc.OptinManaged.password",
			},
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Timeout: 6 * time.Minute,
	})
}

func OptinManaged(ctx context.Context, s *testing.State) {
	const (
		// If a single variant is flaky, please promote this to test params and increase the
		// attempts only for that specific variant instead of updating the constant for all.
		// See crrev.com/c/2979454 for an example.
		maxAttempts = 1
	)

	param := s.Param().(optinTestParam)
	gaiaLogin := chrome.GAIALogin(chrome.Creds{User: s.RequiredVar(param.username), Pass: s.RequiredVar(param.password)})

	fdms, err := fakedms.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start FakeDMS: ", err)
	}
	defer fdms.Stop(ctx)

	if err := fdms.WritePolicyBlob(fakedms.NewPolicyBlob()); err != nil {
		s.Fatal("Failed to write policies to FakeDMS: ", err)
	}

	cr, err := chrome.New(ctx,
		gaiaLogin,
		chrome.DMSPolicy(fdms.URL),
		chrome.ARCSupported(),
		chrome.ExtraArgs(arc.DisableSyncFlags()...))
	if err != nil {
		s.Fatal("Failed to create Chrome")
	}
	defer cr.Close(ctx)

	s.Log("Performing optin")

	if err := policyutil.ServeAndRefresh(ctx, fdms, cr, []policy.Policy{&policy.ArcEnabled{Val: true, Stat: policy.StatusSet}}); err != nil {
		s.Fatal("Failed to update policies: ", err)
	}

	conn, err := cr.TestAPIConn(ctx)
	kb, err := input.Keyboard(ctx)
	if err := launcher.SearchAndLaunch(conn, kb, apps.PlayStore.Name)(ctx); err != nil {
		s.Fatal("Failed to launch Play Store: ", err)
	}
	if err := optin.WaitForPlayStoreShown(ctx, conn, 2*time.Minute); err != nil {
		s.Fatal("Failed to wait for Play Store to show up: ", err)
	}
	if err := ash.WaitForVisible(ctx, conn, playstorePackageName); err != nil {
		s.Fatal("Failed to verify Play Store window is visible: ", err)
	}
}
