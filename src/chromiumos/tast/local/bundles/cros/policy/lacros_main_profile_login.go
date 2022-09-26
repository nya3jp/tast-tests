// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/pci"
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

type testCase struct {
	// The button that should be pressed to accept of decline sync.
	// If not present - FRE is expected to be skipped.
	syncButton func() *nodewith.Finder
	// The sync message that is expected to be shown at the end of the test.
	syncMessage             func() *nodewith.Finder
	enableSyncConsentPolicy *policy.EnableSyncConsent
	syncDisabledPolicy      *policy.SyncDisabled
}

func profileMenu() *nodewith.Finder {
	return nodewith.Name("Settings - Sync and Google services").Role(role.RootWebArea)
}

func syncOnState() *nodewith.Finder {
	// When sync is on - "Turn off" button is shown.
	return nodewith.Name("Turn off").Role(role.Button).Ancestor(profileMenu())
}

func syncOffState() *nodewith.Finder {
	// When sync is off - "Turn on sync" button is shown.
	return nodewith.NameStartingWith("Turn on sync").Role(role.Button).Ancestor(profileMenu())
}

func syncDisabledState() *nodewith.Finder {
	return nodewith.Name("Sync disabled").Role(role.StaticText).Ancestor(profileMenu())
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         LacrosMainProfileLogin,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Browser profile gets auto-created for the user, user is automatically logged into the profile",
		Contacts: []string{
			"anastasiian@chromium.org", // Test author
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Fixture:      fixture.LacrosPolicyLoggedInRealUser,
		Params: []testing.Param{
			// cases with no policy.
			{
				Name: "no_policy_sync_on",
				Val: testCase{
					syncButton:              func() *nodewith.Finder { return nodewith.Name("Yes, I'm in").Role(role.Button) },
					syncMessage:             syncOnState,
					enableSyncConsentPolicy: &policy.EnableSyncConsent{Stat: policy.StatusUnset},
					syncDisabledPolicy:      &policy.SyncDisabled{Stat: policy.StatusUnset},
				},
			},
			{
				Name: "no_policy_sync_off",
				Val: testCase{
					syncButton:              func() *nodewith.Finder { return nodewith.Name("No thanks").Role(role.Button) },
					syncMessage:             syncOffState,
					enableSyncConsentPolicy: &policy.EnableSyncConsent{Stat: policy.StatusUnset},
					syncDisabledPolicy:      &policy.SyncDisabled{Stat: policy.StatusUnset},
				},
			},
			// SyncDisabled = true -> FRE is skipped, sync is disabled.
			{
				Name: "sync_disabled",
				Val: testCase{
					syncMessage:             syncDisabledState,
					enableSyncConsentPolicy: &policy.EnableSyncConsent{Stat: policy.StatusUnset},
					syncDisabledPolicy:      &policy.SyncDisabled{Val: true},
				},
			},
			{
				Name: "sync_disabled_with_sync_consent_disabled",
				Val: testCase{
					syncMessage:             syncDisabledState,
					enableSyncConsentPolicy: &policy.EnableSyncConsent{Val: false},
					syncDisabledPolicy:      &policy.SyncDisabled{Val: true},
				},
			},
			{
				Name: "sync_disabled_with_sync_consent_enabled",
				Val: testCase{
					syncMessage:             syncDisabledState,
					enableSyncConsentPolicy: &policy.EnableSyncConsent{Val: true},
					syncDisabledPolicy:      &policy.SyncDisabled{Val: true},
				},
			},
			// EnableSyncConsent = false -> FRE is skipped, sync is enabled.
			{
				Name: "sync_consent_disabled",
				Val: testCase{
					syncMessage:             syncOnState,
					enableSyncConsentPolicy: &policy.EnableSyncConsent{Val: false},
					syncDisabledPolicy:      &policy.SyncDisabled{Stat: policy.StatusUnset},
				},
			},
			{
				Name: "sync_consent_disabled_with_sync_disabled_false",
				Val: testCase{
					syncMessage:             syncOnState,
					enableSyncConsentPolicy: &policy.EnableSyncConsent{Val: false},
					syncDisabledPolicy:      &policy.SyncDisabled{Val: false},
				},
			}},
		Timeout: chrome.LoginTimeout + time.Minute,
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.EnableSyncConsent{}, pci.VerifiedFunctionalityUI),
			pci.SearchFlag(&policy.SyncDisabled{}, pci.VerifiedFunctionalityUI),
		},
	})
}

func LacrosMainProfileLogin(ctx context.Context, s *testing.State) {
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()
	param := s.Param().(testCase)

	// Reserve 10 seconds for various cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
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
	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "lacros_login_ui_tree")

	ui := uiauto.New(tconn)

	if param.syncButton != nil {
		welcomeButton := nodewith.Name("Let's go").Role(role.Button)
		syncButton := param.syncButton()
		if err := uiauto.Combine("accept or decline sync",
			ui.WaitUntilExists(welcomeButton),
			ui.DoDefaultUntil(welcomeButton, ui.Exists(syncButton)),
			ui.DoDefault(syncButton),
		)(ctx); err != nil {
			s.Fatal("Failed to accept or decline sync: ", err)
		}
	}

	// FRE opens a new tab page in Lacros browser. Wait for the empty tab to load,
	// so that we don't open Settings page before completing FRE (this could lead
	// to a situation when the new tab page opens after we open the Settings page).
	newTabConn, err := lacros.NewConnForTarget(ctx, chrome.MatchTargetURL(chrome.NewTabURL))
	if err != nil {
		s.Fatal("Failed to connect to the new tab: ", err)
	}
	defer newTabConn.Close()

	conn, err := lacros.NewConn(ctx, "chrome://settings/syncSetup")
	if err != nil {
		s.Fatal("Failed to open a new tab in Lacros browser: ", err)
	}
	defer conn.Close()

	syncMessage := param.syncMessage()
	if err := ui.WaitUntilExists(syncMessage)(ctx); err != nil {
		s.Fatal("Failed to find the expected sync message: ", err)
	}
}
