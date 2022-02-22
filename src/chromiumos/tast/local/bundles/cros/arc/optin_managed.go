// Copyright 2021 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/testing"
)

const (
	playstorePackageName    = "com.android.vending"
	username                = "arc.OptinManaged.username"
	password                = "arc.OptinManaged.password"
	timeoutWaitForPlayStore = 4 * time.Minute
)

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
	fdms, err := setupFakePolicyServer(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to setup fake policy server: ", err)
	}
	defer fdms.Stop(ctx)

	// Actual username and password are read from vars/arc.OptinManaged.yaml.
	creds := chrome.Creds{User: s.RequiredVar(username), Pass: s.RequiredVar(password)}
	gaiaLogin := chrome.GAIALogin(creds)

	cr, err := setupManagedChrome(ctx, gaiaLogin, fdms)
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

func setupFakePolicyServer(ctx context.Context, outdir string) (*fakedms.FakeDMS, error) {
	fdms, err := fakedms.New(ctx, outdir)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create fakedms")
	}

	// The policy means to force ARC to be enabled.
	var policies = []policy.Policy{&policy.ArcEnabled{Val: true, Stat: policy.StatusSet}}

	// Add the new policy to fmds
	blob := fakedms.NewPolicyBlob()
	if err := blob.AddPolicies(policies); err != nil {
		fdms.Stop(ctx)
		return nil, errors.Wrap(err, "failed to add policy to policy blob")
	}
	if err := fdms.WritePolicyBlob(blob); err != nil {
		fdms.Stop(ctx)
		return nil, errors.Wrap(err, "failed to write policy blob to fdms")
	}
	return fdms, nil
}

func setupManagedChrome(ctx context.Context, gaiaLogin chrome.Option, fdms *fakedms.FakeDMS) (*chrome.Chrome, error) {
	// If fdms forces ARC opt-in, then ARC opt-in will start in background, right after chrome is created.
	cr, err := chrome.New(ctx,
		gaiaLogin,
		chrome.DMSPolicy(fdms.URL),
		chrome.ARCSupported(),
		chrome.ExtraArgs(arc.DisableSyncFlags()...))
	if err != nil {
		err = errors.Wrap(err, "failed to create Chrome")
		return nil, err
	}

	return cr, nil
}
