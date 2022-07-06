// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LacrosMainProfileLogin,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Browser profile gets auto-created for the user, user is automatically logged into the profile",
		Contacts: []string{
			"anastasiian@chromium.org", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      fixture.LacrosPolicyLoggedInRealUser,
		Timeout:      9*chrome.LoginTimeout + 2*time.Minute,
	})
}

func LacrosMainProfileLogin(ctx context.Context, s *testing.State) {
	profileMenu := nodewith.Name("Settings - Sync and Google services").Role(role.RootWebArea)
	// When sync is on - "Turn off" button is shown.
	syncOnState := nodewith.Name("Turn off").Role(role.Button).Ancestor(profileMenu)
	// When sync is off - "Turn on sync" button is shown.
	syncOffState := nodewith.NameStartingWith("Turn on sync").Role(role.Button).Ancestor(profileMenu)
	syncDisabledState := nodewith.Name("Sync disabled").Role(role.StaticText).Ancestor(profileMenu)
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	for _, param := range []struct {
		// name is the subtest name.
		name string
		// the button that should be pressed to accept of decline sync.
		// if not present - FRE is expected to be skipped.
		syncButton *nodewith.Finder
		// the sync message that is expected to be shown at the end of the test.
		syncMessage             *nodewith.Finder
		enableSyncConsentPolicy *policy.EnableSyncConsent
		syncDisabledPolicy      *policy.SyncDisabled
	}{
		// cases with no policy.
		{
			name:                    "no_policy_sync_on",
			syncButton:              nodewith.Name("Yes, I'm in").Role(role.Button),
			syncMessage:             syncOnState,
			enableSyncConsentPolicy: &policy.EnableSyncConsent{Stat: policy.StatusUnset},
			syncDisabledPolicy:      &policy.SyncDisabled{Stat: policy.StatusUnset},
		},
		{
			name:                    "no_policy_sync_off",
			syncButton:              nodewith.Name("No thanks").Role(role.Button),
			syncMessage:             syncOffState,
			enableSyncConsentPolicy: &policy.EnableSyncConsent{Stat: policy.StatusUnset},
			syncDisabledPolicy:      &policy.SyncDisabled{Stat: policy.StatusUnset},
		},
		// EnableSyncConsent = true is the same as above.
		{
			name:                    "sync_consent_enabled_sync_on",
			syncButton:              nodewith.Name("Yes, I'm in").Role(role.Button),
			syncMessage:             syncOnState,
			enableSyncConsentPolicy: &policy.EnableSyncConsent{Val: true},
			syncDisabledPolicy:      &policy.SyncDisabled{Stat: policy.StatusUnset},
		},
		// SyncDisabled = false is the same as above.
		{
			name:                    "sync_disabled_false_sync_on",
			syncButton:              nodewith.Name("Yes, I'm in").Role(role.Button),
			syncMessage:             syncOnState,
			enableSyncConsentPolicy: &policy.EnableSyncConsent{Stat: policy.StatusUnset},
			syncDisabledPolicy:      &policy.SyncDisabled{Val: false},
		},
		// SyncDisabled = true -> FRE is skipped, sync is disabled.
		{
			name:                    "sync_disabled",
			syncMessage:             syncDisabledState,
			enableSyncConsentPolicy: &policy.EnableSyncConsent{Stat: policy.StatusUnset},
			syncDisabledPolicy:      &policy.SyncDisabled{Val: true},
		},
		{
			name:                    "sync_disabled_with_sync_consent_disabled",
			syncMessage:             syncDisabledState,
			enableSyncConsentPolicy: &policy.EnableSyncConsent{Val: false},
			syncDisabledPolicy:      &policy.SyncDisabled{Val: true},
		},
		{
			name:                    "sync_disabled_with_sync_consent_enabled",
			syncMessage:             syncDisabledState,
			enableSyncConsentPolicy: &policy.EnableSyncConsent{Val: true},
			syncDisabledPolicy:      &policy.SyncDisabled{Val: true},
		},
		// EnableSyncConsent = false -> FRE is skipped, sync is enabled.
		{
			name:                    "sync_consent_disabled",
			syncMessage:             syncOnState,
			enableSyncConsentPolicy: &policy.EnableSyncConsent{Val: false},
			syncDisabledPolicy:      &policy.SyncDisabled{Stat: policy.StatusUnset},
		},
		{
			name:                    "sync_consent_disabled_with_sync_disabled_false",
			syncMessage:             syncOnState,
			enableSyncConsentPolicy: &policy.EnableSyncConsent{Val: false},
			syncDisabledPolicy:      &policy.SyncDisabled{Val: false},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Reserve 30 seconds for various cleanup.
			cleanupCtx := ctx
			ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
			defer cancel()

			// Start chrome.
			cr, err := chrome.New(ctx, s.FixtValue().(*fixtures.PolicyRealUserFixtData).Opts()...)
			if err != nil {
				s.Fatal("Failed to start Chrome: ", err)
			}
			defer cr.Close(cleanupCtx)

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			policies := []policy.Policy{param.enableSyncConsentPolicy, param.syncDisabledPolicy}
			if err := policyutil.ServeAndRefresh(ctx, fdms, cr, policies); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Connect to Test API to use it with the UI library.
			tconn, err := cr.TestAPIConn(ctx)
			if err != nil {
				s.Fatal("Failed to connect Test API: ", err)
			}

			// Launch Lacros.
			lacros, err := lacros.Launch(ctx, tconn)
			if err != nil {
				s.Fatal("Failed to launch lacros-chrome: ", err)
			}
			defer lacros.Close(cleanupCtx)
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, param.name)

			ui := uiauto.New(tconn)

			if param.syncButton != nil {
				welcomeButton := nodewith.Name("Let's go").Role(role.Button)
				if err := uiauto.Combine("accept or decline sync",
					ui.WaitUntilExists(welcomeButton),
					ui.DoDefaultUntil(welcomeButton, ui.Exists(param.syncButton)),
					ui.DoDefault(param.syncButton),
				)(ctx); err != nil {
					s.Fatal("Failed to accept or decline sync: ", err)
				}
			}

			conn, err := lacros.NewConn(ctx, "chrome://settings/syncSetup")
			if err != nil {
				s.Fatal("Failed to open a new tab in Lacros browser: ", err)
			}
			defer conn.Close()

			if err := ui.WaitUntilExists(param.syncMessage)(ctx); err != nil {
				s.Fatal("Failed to find the expected sync message: ", err)
			}
		})
	}
}
