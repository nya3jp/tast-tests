// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

type testParams struct {
	username   string // username for Chrome login.
	password   string // password to login.
	arcEnabled bool   // value of ArcEnabled user policy.
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         EnterpriseLogin,
		Desc:         "Checks that ARC is launched when policy is set",
		Contacts:     []string{"pbond@chromium.org", "arc-commercial@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"arc", "chrome"},
		Timeout:      8 * time.Minute,
		Params: []testing.Param{
			{
				Name: "managed_3pp_true",
				Val: testParams{
					username:   "arc.EnterpriseLogin.managed_3pp_true_user",
					password:   "arc.EnterpriseLogin.managed_3pp_true_password",
					arcEnabled: true,
				}},
			{
				Name: "managed_3pp_false",
				Val: testParams{
					username:   "arc.EnterpriseLogin.managed_3pp_false_user",
					password:   "arc.EnterpriseLogin.managed_3pp_false_password",
					arcEnabled: false,
				}},
			{
				Name: "managed_necktie_false",
				Val: testParams{
					username:   "arc.EnterpriseLogin.managed_necktie_false_user",
					password:   "arc.EnterpriseLogin.managed_necktie_false_password",
					arcEnabled: false,
				}},
			{
				Name: "managed_necktie_true",
				Val: testParams{
					username:   "arc.EnterpriseLogin.managed_necktie_true_user",
					password:   "arc.EnterpriseLogin.managed_necktie_true_password",
					arcEnabled: true,
				}},
			{
				Name: "managed_unmanaged_false",
				Val: testParams{
					username:   "arc.EnterpriseLogin.managed_unmanaged_false_user",
					password:   "arc.EnterpriseLogin.managed_unmanaged_false_password",
					arcEnabled: false,
				}},
			{
				Name: "managed_unmanaged_true",
				Val: testParams{
					username:   "arc.EnterpriseLogin.managed_unmanaged_true_user",
					password:   "arc.EnterpriseLogin.managed_unmanaged_true_password",
					arcEnabled: true,
				}}},
		Vars: []string{
			"arc.EnterpriseLogin.managed_3pp_true_user",
			"arc.EnterpriseLogin.managed_3pp_true_password",
			"arc.EnterpriseLogin.managed_3pp_false_user",
			"arc.EnterpriseLogin.managed_3pp_false_password",
			"arc.EnterpriseLogin.managed_necktie_false_user",
			"arc.EnterpriseLogin.managed_necktie_false_password",
			"arc.EnterpriseLogin.managed_necktie_true_user",
			"arc.EnterpriseLogin.managed_necktie_true_password",
			"arc.EnterpriseLogin.managed_unmanaged_false_user",
			"arc.EnterpriseLogin.managed_unmanaged_false_password",
			"arc.EnterpriseLogin.managed_unmanaged_true_user",
			"arc.EnterpriseLogin.managed_unmanaged_true_password",
		},
	})
}

func EnterpriseLogin(ctx context.Context, s *testing.State) {
	param := s.Param().(testParams)
	username := s.RequiredVar(param.username)
	password := s.RequiredVar(param.password)
	arcEnabled := param.arcEnabled

	// Log-in to Chrome and allow to launch ARC if allowed by user policy.
	cr, err := chrome.New(
		ctx,
		chrome.Auth(username, password, "gaia-id"),
		chrome.GAIALogin(),
		chrome.ARCSupported(),
		chrome.ProdPolicy())
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Ensure chrome://policy shows correct ArcEnabled value.
	if err := policyutil.Verify(ctx, tconn, []policy.Policy{&policy.ArcEnabled{Val: arcEnabled}}); err != nil {
		s.Fatal("Failed to verify ArcEnabled: ", err)
	}

	if arcEnabled != true {
		ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
		defer cancel()

		// Try to launch ARC, it should not succeed in 120s.
		a, err := arc.New(ctx, s.OutDir())
		defer a.Close()
		if err == nil {
			s.Fatal("Started ARC while blocked by user policy")
		}
		return
	}
	// Ensure that ARC is launched.
	a, err := arc.New(ctx, s.OutDir())
	defer a.Close()
	if err != nil {
		s.Fatal("Failed to start ARC by user policy: ", err)
	}
}
