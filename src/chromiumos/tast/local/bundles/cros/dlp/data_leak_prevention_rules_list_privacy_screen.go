// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dlp

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DataLeakPreventionRulesListPrivacyScreen,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Test behavior of DataLeakPreventionRulesList policy with privacy screen blocked restriction",
		Contacts: []string{
			"ayaelattar@google.com",
			"chromeos-dlp@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.PrivacyScreen()),
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Fixture: fixture.ChromePolicyLoggedIn,
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val:               browser.TypeLacros,
		}},
	})
}

func DataLeakPreventionRulesListPrivacyScreen(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fakeDMS := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// DLP policy with privacy screen blocked restriction.
	policyDLP := []policy.Policy{&policy.DataLeakPreventionRulesList{
		Val: []*policy.DataLeakPreventionRulesListValue{
			{
				Name:        "Enable privacy screen for confidential content in restricted source",
				Description: "Privacy screen should be enabled when on restricted site",
				Sources: &policy.DataLeakPreventionRulesListValueSources{
					Urls: []string{
						"example.com",
					},
				},
				Restrictions: []*policy.DataLeakPreventionRulesListValueRestrictions{
					{
						Class: "PRIVACY_SCREEN",
						Level: "BLOCK",
					},
				},
			},
		},
	},
	}

	// Update the policy blob.
	pb := policy.NewBlob()
	pb.AddPolicies(policyDLP)

	// Update policy.
	if err := policyutil.ServeBlobAndRefresh(ctx, fakeDMS, cr, pb); err != nil {
		s.Fatal("Failed to serve and refresh: ", err)
	}

	// Connect to Test API.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	for _, param := range []struct {
		name        string
		wantAllowed bool
		url         string
	}{
		{
			name:        "example",
			wantAllowed: false,
			url:         "www.example.com",
		},
		{
			name:        "chromium",
			wantAllowed: true,
			url:         "www.chromium.org",
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			br, closeBrowser, err := browserfixt.SetUp(ctx, cr, s.Param().(browser.Type))
			if err != nil {
				s.Fatal("Failed to open the browser: ", err)
			}
			defer closeBrowser(cleanupCtx)

			ui := uiauto.New(tconn)

			conn, err := br.NewConn(ctx, "https://"+param.url)
			if err != nil {
				s.Fatal("Failed to open page: ", err)
			}
			defer conn.Close()

			defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			if err := checkPrivacyScreenOnBubble(ctx, ui, param.wantAllowed); err != nil {
				s.Error("Couldn't check for notification: ", err)
			}

			value, err := privacyScreenValue(ctx)
			if err != nil {
				s.Fatal("Couldn't check value for privacy screen prop: ", err)
			}

			if !param.wantAllowed && !value {
				s.Errorf("Privacy screen prop value: got %v; want true", value)
			}

			if param.wantAllowed && value {
				s.Errorf("Privacy screen prop value: got %v; want false", value)
			}

			if _, err := br.NewConn(ctx, "https://www.google.com"); err != nil {
				s.Error("Failed to open page: ", err)
			}

			if err := checkPrivacyScreenOffBubble(ctx, ui, param.wantAllowed); err != nil {
				s.Error("Couldn't check for notification: ", err)
			}

			// Wait for privacy screen to be disabled.
			if err := testing.Sleep(ctx, time.Second); err != nil {
				s.Fatal("Failed to sleep: ", err)
			}

			value, err = privacyScreenValue(ctx)
			// Privacy screen should be disabled.
			if value {
				s.Errorf("Privacy screen prop value: got %v; want false", value)
			}
		})
	}
}

func checkPrivacyScreenOnBubble(ctx context.Context, ui *uiauto.Context, wantAllowed bool) error {
	// Message name - IDS_ASH_STATUS_TRAY_PRIVACY_SCREEN_TOAST_ACCESSIBILITY_TEXT
	bubbleMessage := nodewith.NameContaining("Privacy screen is on. Enforced by your administrator").First()

	err := ui.WaitUntilExists(bubbleMessage)(ctx)

	if err != nil && !wantAllowed {
		return errors.Wrap(err, "failed to check for privacy screen on bubble")
	}

	if err == nil && wantAllowed {
		return errors.New("Privacy screen on bubble found expected none")
	}

	return nil
}

func checkPrivacyScreenOffBubble(ctx context.Context, ui *uiauto.Context, wantAllowed bool) error {
	// Message name - IDS_ASH_STATUS_TRAY_PRIVACY_SCREEN_OFF_STATE
	bubbleMessage := nodewith.NameContaining("Privacy screen is off").First()

	err := ui.WaitUntilExists(bubbleMessage)(ctx)

	if err != nil && !wantAllowed {
		return errors.Wrap(err, "failed to check for privacy screen off bubble bubble existence")
	}

	if err == nil && wantAllowed {
		return errors.New("Privacy screen off bubble found expected none")
	}

	return nil
}

// privacyScreenValue retrieves value of privacy screen prop.
func privacyScreenValue(ctx context.Context) (bool, error) {
	// modetest -c get list of connectors
	output, err := testexec.CommandContext(ctx, "modetest", "-c").Output()
	if err != nil {
		return false, err
	}

	// Get privacy-screen connector.
	outputSlice := strings.Split(string(output), "privacy-screen:")

	if len(outputSlice) <= 1 {
		return false, errors.New("failed to find privacy screen prop")
	}

	for _, line := range strings.Split(outputSlice[1], "\n") {
		// Check for prop value.
		matches := strings.Contains(line, "value:")
		if matches {
			if found := strings.Contains(line, "1"); found {
				return true, nil
			}

			if found := strings.Contains(line, "0"); found {
				return false, nil
			}

			// Need to check for prop value only once.
			return false, errors.New("failed to find value for privacy screen prop")
		}
	}

	return false, nil
}
