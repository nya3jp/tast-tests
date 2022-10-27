// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/bundles/cros/arc/oobeutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

type oobeArcTestOptions struct {
	consolidatedConsentEnabled bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         OobeArc,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Navigate through OOBE and Verify that PlayStore Settings Screen is launched at the end",
		Contacts:     []string{"cpiao@google.com", "cros-arc-te@google.com", "cros-oac@google.com", "cros-oobe@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:arc-functional"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
			Val: oobeArcTestOptions{
				consolidatedConsentEnabled: false,
			},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Val: oobeArcTestOptions{
				consolidatedConsentEnabled: false,
			},
		}, {
			Name:              "p_consolidated_consent",
			ExtraSoftwareDeps: []string{"android_p"},
			Val: oobeArcTestOptions{
				consolidatedConsentEnabled: true,
			},
		}, {
			Name:              "vm_consolidated_consent",
			ExtraSoftwareDeps: []string{"android_vm"},
			Val: oobeArcTestOptions{
				consolidatedConsentEnabled: true,
			},
		}},
		Timeout: chrome.GAIALoginTimeout + arc.BootTimeout + 10*time.Minute,
		VarDeps: []string{"ui.gaiaPoolDefault"},
	})
}

func OobeArc(ctx context.Context, s *testing.State) {
	testOptions := s.Param().(oobeArcTestOptions)
	chromeOptions := []chrome.Option{
		chrome.DontSkipOOBEAfterLogin(),
		chrome.ARCSupported(),
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
	}
	if testOptions.consolidatedConsentEnabled {
		chromeOptions = append(chromeOptions, chrome.EnableFeatures("OobeConsolidatedConsent", "PerUserMetricsConsent"))
	} else {
		chromeOptions = append(chromeOptions, chrome.DisableFeatures("OobeConsolidatedConsent", "PerUserMetricsConsent"))
	}

	cr, err := chrome.New(ctx, chromeOptions...)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
	ui := uiauto.New(tconn)

	if testOptions.consolidatedConsentEnabled {
		if err := oobeutil.CompleteConsolidatedConsentOnboardingFlow(ctx, ui); err != nil {
			s.Fatal("Failed to go through the oobe flow: ", err)
		}
	} else {
		if err := oobeutil.CompleteRegularOnboardingFlow(ctx, ui /*reviewArcOptions=*/, true); err != nil {
			s.Fatal("Failed to go through the oobe flow: ", err)
		}
	}

	if err := oobeutil.CompleteTabletOnboarding(ctx, ui); err != nil {
		s.Fatal("Failed to test oobe Arc tablet flow: ", err)
	}

	s.Log("Verify Play Store is On")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		playStoreState, err := optin.GetPlayStoreState(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get some playstore state")
		}
		if playStoreState["enabled"] == false {
			return errors.New("playstore is off")
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		s.Fatal("Failed to verify Play Store is On: ", err)
	}

	if !testOptions.consolidatedConsentEnabled {
		s.Log("Verify Play Store Settings is Launched")
		if err := ui.WaitUntilExists(nodewith.Name("Remove Google Play Store").Role(role.Button))(ctx); err != nil {
			s.Fatal("Failed to Launch Android Settings After OOBE Flow : ", err)
		}
	}
}
