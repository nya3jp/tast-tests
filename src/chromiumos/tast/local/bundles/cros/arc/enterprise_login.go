// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/tape"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

const enterpriseLoginTestTimeout = 8 * time.Minute

type testParams struct {
	poolID     string // id of the account pool used for Chrome login.
	arcEnabled bool   // arcEnabled is the value of ArcEnabled user policy.
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         EnterpriseLogin,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that ARC is launched when policy is set",
		Contacts:     []string{"mhasank@chromium.org", "arc-commercial@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"arc", "chrome"},
		Timeout:      enterpriseLoginTestTimeout,
		Params: []testing.Param{
			{
				Name: "managed_unmanaged_false",
				Val: testParams{
					poolID:     "arc_enterprise_login_managed_unmanaged_false",
					arcEnabled: false,
				}},
			{
				Name: "managed_unmanaged_true",
				Val: testParams{
					poolID:     "arc_enterprise_login_managed_unmanaged_true",
					arcEnabled: true,
				}}},
		VarDeps: []string{"tape.service_account_key"},
	})
}

func EnterpriseLogin(ctx context.Context, s *testing.State) {
	const (
		cleanupTime = 1 * time.Minute // time reserved for cleanup.
	)

	param := s.Param().(testParams)
	arcEnabled := param.arcEnabled

	// Use a shortened context for test operations to reserve time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, cleanupTime)
	defer cancel()

	// Create an account manager and lease a test account for the duration of the test.
	accHelper, acc, err := tape.NewOwnedTestAccountManager(ctx, []byte(s.RequiredVar(tape.ServiceAccountVar)), false, tape.WithTimeout(int32(enterpriseLoginTestTimeout.Seconds())), tape.WithPoolID(param.poolID))
	if err != nil {
		s.Fatal("Failed to create an account manager and lease an account: ", err)
	}
	defer accHelper.CleanUp(cleanupCtx)

	// Log-in to Chrome and allow to launch ARC if allowed by user policy.
	cr, err := chrome.New(
		ctx,
		chrome.GAIALogin(chrome.Creds{User: acc.Username, Pass: acc.Password}),
		chrome.ARCSupported(),
		chrome.UnRestrictARCCPU(),
		// TODO(b/154760453): switch to fake DMS once crbug.com/1099310 is resolved
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

	// Try to launch ARC, it should succeed only if enabled by policy.
	a, err := arc.New(ctx, s.OutDir())
	if err == nil {
		defer a.Close(ctx)
		if arcEnabled != true {
			s.Error("Started ARC while blocked by user policy")
		}
	}

	if arcEnabled == true && err != nil {
		s.Error("Failed to start ARC by user policy: ", err)
	}
}
