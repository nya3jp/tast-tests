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
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

const (
	playstorePackageName = "com.android.vending"
	username             = "arc.OptinManaged.username"
	password             = "arc.OptinManaged.password"
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

	cr, err := setupManagedChrome(ctx, gaiaLogin)
	if err != nil {
		s.Fatal("Failed to setup chrome: ", err)
	}
	defer cr.Close(ctx)

	s.Log("Performing optin")

	// The policy means to force ARC to be enabled. After |ServeAndRefresh|, ARC should already be opted in.
	var policy = []policy.Policy{&policy.ArcEnabled{Val: true, Stat: policy.StatusSet}}
	if err := policyutil.ServeAndRefresh(ctx, fdms, cr, policy); err != nil {
		s.Fatal("Failed to update policies: ", err)
	}

	// Verify that ARC opt-in is successful, by trying to open Play Store and checking that its window is shown.
	conn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API Conn: ", err)
	}
	if err := apps.Launch(ctx, conn, apps.PlayStore.ID); err != nil {
		s.Fatal("Failed to launch Play Store: ", err)
	}
	if err := optin.WaitForPlayStoreShown(ctx, conn, 4*time.Minute); err != nil {
		s.Fatal("Failed to wait for Play Store to show up: ", err)
	}
}

func setupFakePolicyServer(ctx context.Context, outdir string) (*fakedms.FakeDMS, error) {
	fdms, err := fakedms.New(ctx, outdir)
	if err != nil {
		return nil, err
	}

	if err := fdms.WritePolicyBlob(fakedms.NewPolicyBlob()); err != nil {
		return fdms, err
	}

	return fdms, nil
}

func setupManagedChrome(ctx context.Context, gaiaLogin chrome.Option) (*chrome.Chrome, error) {
	cr, err := chrome.New(ctx,
		gaiaLogin,
		chrome.ARCSupported(),
		chrome.ExtraArgs(arc.DisableSyncFlags()...))
	if err != nil {
		return nil, err
	}

	return cr, nil
}
