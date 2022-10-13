// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

const (
	loginPoolVar = "arc.managedAccountPool"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         OptinManaged,
		LacrosStatus: testing.LacrosVariantUnneeded,
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
		Timeout: 6 * time.Minute,
	})
}

func OptinManaged(ctx context.Context, s *testing.State) {
	const (
		bootTimeout         = 4 * time.Minute
		provisioningTimeout = 3 * time.Minute
	)

	// Actual username and password are read from vars/arc.yaml.
	creds, err := chrome.PickRandomCreds(s.RequiredVar(loginPoolVar))
	if err != nil {
		s.Fatal("Failed to get login creds: ", err)
	}

	// The policy means to force ARC to be enabled.
	policies := []policy.Policy{&policy.ArcEnabled{Val: true, Stat: policy.StatusSet}}

	fdms, err := policyutil.SetUpFakePolicyServer(ctx, s.OutDir(), creds.User, policies)
	if err != nil {
		s.Fatal("Failed to setup fake policy server: ", err)
	}
	defer fdms.Stop(ctx)

	gaiaLogin := chrome.GAIALogin(creds)
	cr, err := setupManagedChrome(ctx, gaiaLogin, fdms)
	if err != nil {
		s.Fatal("Failed to setup chrome: ", err)
	}
	defer cr.Close(ctx)

	s.Log("Waiting for managed provisioning")

	a, err := arc.NewWithTimeout(ctx, s.OutDir(), bootTimeout)
	if err != nil {
		s.Fatal("Failed to start ARC by policy: ", err)
	}
	defer a.Close(ctx)

	if err := a.WaitForProvisioning(ctx, provisioningTimeout); err != nil {
		if err := optin.DumpLogCat(ctx, ""); err != nil {
			s.Logf("WARNING: Failed to dump logcat: %s", err)
		}
		s.Fatal("Managed provisioning failed: ", err)
	}
}

func setupManagedChrome(ctx context.Context, gaiaLogin chrome.Option, fdms *fakedms.FakeDMS) (*chrome.Chrome, error) {
	// If fdms forces ARC opt-in, then ARC opt-in will start in background, right after chrome is created.
	cr, err := chrome.New(ctx,
		gaiaLogin,
		chrome.DMSPolicy(fdms.URL),
		chrome.ARCSupported(),
		chrome.UnRestrictARCCPU(),
		chrome.ExtraArgs(arc.DisableSyncFlags()...))
	if err != nil {
		err = errors.Wrap(err, "failed to create Chrome")
		return nil, err
	}

	return cr, nil
}
