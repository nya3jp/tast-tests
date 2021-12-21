// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package borealis

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/dlc"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		// TODO(b/211514314): Add HardwareDeps to limit this test to high powered devices only.
		Func:         DevicePolicy,
		Desc:         "Attempt to install Borealis with varying policy configurations",
		Contacts:     []string{"hollingum@google.com", "borealis-eng@google.com"},
		Attr:         []string{"group:borealis", "borealis_perbuild"},
		SoftwareDeps: []string{"chrome", "borealis_host"},
		Timeout:      3 * time.Minute,
		Fixture:      fixture.FakeDMSEnrolled,
	})
}

func checkWithUI(ctx context.Context, tconn *chrome.TestConn, shouldInstall bool) error {
	// TODO(b/211514314): Use borealis.InstallWithoutLaunch().
	canInstall := tconn.Eval(ctx, `tast.promisify(chrome.autotestPrivate.installBorealis)()`, nil) == nil
	if canInstall != shouldInstall {
		return errors.Errorf("able to install: expected %t, got %t", shouldInstall, canInstall)
	}
	return nil
}

func checkWithCLI(ctx context.Context, cr *chrome.Chrome, shouldInstall bool) error {
	// TODO(b/211514314): Use borealis.InstallWithoutChrome().
	if err := dlc.Install(ctx, "borealis-dlc", "" /*omahaURL*/); err != nil {
		return err
	}
	chromeUserHash, err := cryptohome.UserHash(ctx, cr.NormalizedUser())
	if err != nil {
		return err
	}
	cmd := testexec.CommandContext(ctx, "env", "CROS_USER_ID_HASH="+chromeUserHash, "vmc", "start", "--dlc-id=borealis-dlc", "--no-start-lxd", "foobar")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return errors.Wrap(err, "failed to open stdin pipe")
	}
	if err := cmd.Start(); err != nil {
		return errors.Wrapf(err, "failed to start command %v", cmd)
	}
	if _, err := stdin.Write([]byte("exit\n")); err != nil {
		return errors.Wrap(err, "failed to write 'exit' to stdin pipe")
	}

	canInstall := cmd.Wait(testexec.DumpLogOnError) == nil
	if canInstall != shouldInstall {
		return errors.Errorf("able to install: expected %t, got %t", shouldInstall, canInstall)
	}
	return nil
}

func DevicePolicy(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(cleanupCtx, 30*time.Second)
	defer cancel()

	fdms := s.FixtValue().(*fakedms.FakeDMS)
	cr, err := chrome.New(ctx,
		chrome.FakeLogin(chrome.Creds{User: fixtures.Username, Pass: fixtures.Password}),
		chrome.DMSPolicy(fdms.URL),
		chrome.KeepEnrollment())
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	for _, param := range []struct {
		name               string // Used to identify this subtest in logs
		shouldAllowInstall bool
		policies           []policy.Policy
	}{
		{
			name:               "Default",
			shouldAllowInstall: false,
			policies:           []policy.Policy{},
		},
		{
			name:               "Device disabled",
			shouldAllowInstall: false,
			policies:           []policy.Policy{&policy.DeviceBorealisAllowed{Val: false}},
		},
		{
			name:               "Device enabled User unspecified",
			shouldAllowInstall: false,
			policies:           []policy.Policy{&policy.DeviceBorealisAllowed{Val: true}},
		},
		{
			name:               "Device enabled User disabled",
			shouldAllowInstall: false,
			policies:           []policy.Policy{&policy.DeviceBorealisAllowed{Val: true}, &policy.UserBorealisAllowed{Val: false}},
		},
		{
			name:               "Allowed",
			shouldAllowInstall: true,
			policies:           []policy.Policy{&policy.DeviceBorealisAllowed{Val: true}, &policy.UserBorealisAllowed{Val: true}},
		},
	} {
		s.Logf("Running with policy configuration %q", param.name)

		pb := fakedms.NewPolicyBlob()
		pb.AddPolicies(param.policies)
		if err := policyutil.ServeBlobAndRefresh(ctx, fdms, cr, pb); err != nil {
			s.Fatalf("Unable to set policies for %q: %v", param.name, err)
		}

		if err := checkWithCLI(ctx, cr, param.shouldAllowInstall); err != nil {
			s.Fatalf("Problem with CLI for %q: %v", param.name, err)
		}

		if err := checkWithUI(ctx, tconn, param.shouldAllowInstall); err != nil {
			s.Fatalf("Problem with UI for %q: %v", param.name, err)
		}
	}
}
