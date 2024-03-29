// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package enterprise

import (
	"context"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/arcent"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ARCProvisioning,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "A functional test that verifies provisioning flow for managed user",
		Contacts: []string{
			"arc-commercial@google.com",
			"mhasank@chromium.org",
			"yaohuali@google.com",
		},
		Attr: []string{"group:mainline", "group:arc-functional"},
		VarDeps: []string{
			arcent.LoginPoolVar,
		},
		SoftwareDeps: []string{
			"chrome",
			"chrome_internal",
			"play_store",
		},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p", "no_qemu"},
		}, {
			Name:              "betty",
			ExtraSoftwareDeps: []string{"android_p", "qemu"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm", "no_qemu"},
		}, {
			Name:              "vm_betty",
			ExtraSoftwareDeps: []string{"android_vm", "qemu"},
		}},
		Timeout: 7 * time.Minute,
	})
}

// ARCProvisioning verifies that ARC can successfully provision with a managed account.
func ARCProvisioning(ctx context.Context, s *testing.State) {
	const (
		bootTimeout         = 4 * time.Minute
		provisioningTimeout = 3 * time.Minute
	)

	creds, err := chrome.PickRandomCreds(s.RequiredVar(arcent.LoginPoolVar))
	if err != nil {
		s.Fatal("Failed to get login creds: ", err)
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Minute)
	defer cancel()

	policies := []policy.Policy{&policy.ArcEnabled{Val: true, Stat: policy.StatusSet}}
	fdms, err := policyutil.SetUpFakePolicyServer(ctx, s.OutDir(), creds.User, policies)
	if err != nil {
		s.Fatal("Failed to setup fake policy server: ", err)
	}
	defer fdms.Stop(cleanupCtx)

	gaiaLogin := chrome.GAIALogin(creds)
	cr, err := chrome.New(ctx,
		gaiaLogin,
		chrome.DMSPolicy(fdms.URL),
		chrome.ARCSupported(),
		chrome.UnRestrictARCCPU(),
		chrome.ExtraArgs(arc.DisableSyncFlags()...))
	if err != nil {
		s.Fatal("Failed to setup chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	s.Log("Waiting for managed provisioning")

	a, err := arc.NewWithTimeout(ctx, s.OutDir(), bootTimeout)
	if err != nil {
		s.Fatal("Failed to start ARC by policy: ", err)
	}
	defer a.Close(cleanupCtx)

	if err := a.WaitForProvisioning(ctx, provisioningTimeout); err != nil {
		if err := optin.DumpLogCat(cleanupCtx, ""); err != nil {
			s.Logf("WARNING: Failed to dump logcat: %s", err)
		}
		s.Fatal("Managed provisioning failed: ", err)
	}
}
