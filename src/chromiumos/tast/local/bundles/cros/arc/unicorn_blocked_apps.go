// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/arcent"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/familylink"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         UnicornBlockedApps,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks if blocked apps cannot be installed from Child Account",
		Contacts:     []string{"mhasank@google.com", "arc-commercial@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:arc-functional"},
		SoftwareDeps: []string{
			"chrome",
			"chrome_internal",
			"play_store",
		},
		Timeout: 7 * time.Minute,
		VarDeps: []string{"arc.parentUser", "arc.parentPassword", "arc.childUser", "arc.childPassword"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Fixture: "familyLinkUnicornArcPolicyLogin",
	})
}

func UnicornBlockedApps(ctx context.Context, s *testing.State) {
	const (
		bootTimeout         = 4 * time.Minute
		provisioningTimeout = 3 * time.Minute
		blockedPackage      = "com.google.android.apps.youtube.creator"
	)
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	tconn := s.FixtValue().(familylink.HasTestConn).TestConn()

	packages := []string{blockedPackage}

	arcPolicy := arcent.CreateArcPolicyWithApps(packages, arcent.InstallTypeBlocked)
	arcPolicy.Val.PlayStoreMode = arcent.PlayStoreModeBlockList
	arcEnabledPolicy := &policy.ArcEnabled{Val: true}
	policies := []policy.Policy{arcEnabledPolicy, arcPolicy}

	pb := policy.NewBlob()
	pb.PolicyUser = s.FixtValue().(familylink.HasPolicyUser).PolicyUser()
	pb.AddPolicies(policies)
	if err := policyutil.ServeBlobAndRefresh(ctx, fdms, cr, pb); err != nil {
		s.Fatal("Failed to serve policies: ", err)
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Minute)
	defer cancel()

	a, err := arc.NewWithTimeout(ctx, s.OutDir(), bootTimeout)
	if err != nil {
		s.Fatal("Failed to connect to ARC: ", err)
	}
	defer a.Close(cleanupCtx)

	if err := arcent.ConfigureProvisioningLogs(ctx, a); err != nil {
		s.Fatal("Failed to configure provisioning logs: ", err)
	}

	if err := a.WaitForProvisioning(ctx, provisioningTimeout); err != nil {
		s.Fatal("Failed to wait for provisioning: ", err)
	}

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(cleanupCtx)

	defer arcent.DumpBugReportOnError(cleanupCtx, s.HasError, a, filepath.Join(s.OutDir(), "bugreport.zip"))

	if err := arcent.PollAppPageState(ctx, tconn, a, blockedPackage, func(ctx context.Context) error {
		installButton, err := arcent.WaitForInstallButton(ctx, d)
		if err != nil {
			return errors.Wrap(err, "failed to find the install button")
		}

		enabled, err := installButton.IsEnabled(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to check the install button state")
		}

		if !enabled {
			testing.ContextLog(ctx, "Install button is disabled")
			return nil
		}

		if err := validateAutoUninstall(ctx, a, installButton, blockedPackage); err != nil {
			testing.PollBreak(err)
		}

		testing.ContextLog(ctx, "Blocked app uninstalled")
		return nil
	}, time.Minute); err != nil {
		s.Fatal("Blocked app verification failed: ", err)
	}
}

func validateAutoUninstall(ctx context.Context, a *arc.ARC, installButton *ui.Object, blockedPackage string) error {
	testing.ContextLog(ctx, "Install button is enabled. Attempting install")
	if err := installButton.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click the install button")
	}

	if err := a.WaitForPackages(ctx, []string{blockedPackage}); err != nil {
		return errors.Wrap(err, "package installation failed")
	}

	testing.ContextLog(ctx, "Waiting for package to uninstall")
	if err := waitForUninstall(ctx, a, blockedPackage); err != nil {
		return errors.Wrap(err, "package not uninstalled")
	}

	return nil
}

func waitForUninstall(ctx context.Context, a *arc.ARC, blockedPackage string) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		if installed, err := a.PackageInstalled(ctx, blockedPackage); err != nil {
			return testing.PollBreak(err)
		} else if installed {
			return errors.New("Package not yet uninstalled")
		}
		return nil
	}, &testing.PollOptions{Interval: time.Second})
}
