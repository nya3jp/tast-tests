// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package enterprise

import (
	"context"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/enterprise/arcent"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

const (
	arcEnabled  = true
	arcDisabled = false
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ARCBoot,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that ARC is booted when policy is set",
		Contacts:     []string{"mhasank@chromium.org", "arc-commercial@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "play_store"},
		Timeout:      4 * time.Minute,
		VarDeps: []string{
			arcent.LoginPoolVar,
		},
		Params: []testing.Param{
			{
				Name:              "disabled",
				Val:               arcDisabled,
				ExtraSoftwareDeps: []string{"android_p"},
			},
			{
				Name:              "disabled_vm",
				Val:               arcDisabled,
				ExtraSoftwareDeps: []string{"android_vm"},
			},
			{
				Name:              "enabled",
				Val:               arcEnabled,
				ExtraSoftwareDeps: []string{"android_p"},
			},
			{
				Name:              "enabled_vm",
				Val:               arcEnabled,
				ExtraSoftwareDeps: []string{"android_vm"},
			}},
	})
}

// ARCBoot verifies that ARC boots when enabled in policy and does not boot when disabled in policy.
func ARCBoot(ctx context.Context, s *testing.State) {
	expectEnabled := s.Param().(bool)

	creds, err := chrome.PickRandomCreds(s.RequiredVar(arcent.LoginPoolVar))
	if err != nil {
		s.Fatal("Failed to get login creds: ", err)
	}
	login := chrome.GAIALogin(creds)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Minute)
	defer cancel()

	policies := []policy.Policy{&policy.ArcEnabled{Val: expectEnabled}}
	fdms, err := policyutil.SetUpFakePolicyServer(ctx, s.OutDir(), creds.User, policies)
	if err != nil {
		s.Fatal("Failed to setup fake policy server: ", err)
	}
	defer fdms.Stop(cleanupCtx)

	cr, err := chrome.New(
		ctx,
		login,
		chrome.ARCSupported(),
		chrome.UnRestrictARCCPU(),
		chrome.DMSPolicy(fdms.URL),
		chrome.ExtraArgs(arc.DisableSyncFlags()...))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	// Ensure chrome://policy shows correct ArcEnabled value.
	if err := policyutil.Verify(ctx, tconn, []policy.Policy{&policy.ArcEnabled{Val: expectEnabled}}); err != nil {
		s.Fatal("Failed to verify ArcEnabled: ", err)
	}

	// Wait for ARC to boot. It should succeed only if enabled by policy.
	a, err := arc.New(ctx, s.OutDir())
	if err == nil {
		defer a.Close(ctx)
		if !expectEnabled {
			s.Fatal("Started ARC while blocked by user policy")
		}
	} else if expectEnabled {
		s.Fatal("Failed to start ARC by user policy: ", err)
	}
}
