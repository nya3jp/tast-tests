// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         OptinNetworkError,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "A functional test that validates the 'Check Network' button in optin dialog",
		Contacts: []string{
			"arc-core@google.com",
			"mhasank@chromium.org", // author.
		},
		Attr:    []string{"group:mainline", "group:arc-functional"},
		VarDeps: []string{"ui.gaiaPoolDefault"},
		SoftwareDeps: []string{
			"chrome",
			"chrome_internal",
			"play_store",
		},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Timeout: 6 * time.Minute,
	})
}

// OptinNetworkError tests optin flow with network error.
func OptinNetworkError(ctx context.Context, s *testing.State) {
	cr, err := setupChromeForOptinNetworkError(ctx, s)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	defer modifyArcNetworkDropRule(ctx, "-D" /*op*/)
	s.Log("Blocking ARC network traffic")
	if errs := modifyArcNetworkDropRule(ctx, "-I" /*op*/); len(errs) != 0 {
		s.Fatal("Failed to block ARC network traffic: ", errs)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	s.Log("Performing optin")
	if err := optin.Perform(ctx, cr, tconn); err == nil {
		s.Fatal("Optin succeeded without network: ", err)
	}

	if err := validateCheckNetworkButton(ctx, cr); err != nil {
		s.Fatal("Check network button validation failed: ", err)
	}
}

// validateCheckNetworkButton ensures that the check network button is shown and is working.
func validateCheckNetworkButton(ctx context.Context, cr *chrome.Chrome) error {
	bgURL := chrome.ExtensionBackgroundPageURL(apps.PlayStore.ID)
	conn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(bgURL))
	if err != nil {
		return errors.Wrap(err, "failed to find optin extension page")
	}
	defer conn.Close()

	var btnDisplay string
	if err := conn.Eval(ctx, "appWindow.contentWindow.document.getElementById('button-run-network-tests')?.computedStyleMap().get('display').toString() ?? 'none'", &btnDisplay); err != nil {
		return errors.Wrap(err, "failed to check the button state")
	}

	if btnDisplay == "none" {
		return errors.New("check network button not visible")
	}

	testing.ContextLog(ctx, "Found check network button. Launching Diagnostics app")
	if err := conn.Eval(ctx, "appWindow.contentWindow.document.getElementById('button-run-network-tests').click()", nil); err != nil {
		return errors.Wrap(err, "failed to click the button")
	}

	if err := waitForDiagnosticsApp(ctx, cr, time.Second*5); err != nil {
		return errors.Wrap(err, "Diagnostics app not launched")
	}

	return nil
}

// waitForDiagnosticsApp waits for the diagnostics app to load.
func waitForDiagnosticsApp(ctx context.Context, cr *chrome.Chrome, timeout time.Duration) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		if found, err := cr.IsTargetAvailable(ctx, chrome.MatchTargetURL("chrome://diagnostics/?connectivity")); err != nil {
			return testing.PollBreak(err)
		} else if !found {
			return errors.New("app is not launched yet")
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout})
}

// setupChromeForOptinNetworkError starts chrome with pooled GAIA account and ARC enabled.
func setupChromeForOptinNetworkError(ctx context.Context, s *testing.State) (*chrome.Chrome, error) {
	cr, err := chrome.New(ctx,
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
		chrome.ARCSupported(),
		chrome.EnableFeatures("ButtonARCNetworkDiagnostics", "DiagnosticsAppNavigation", "EnableNetworkingInDiagnosticsApp"))
	return cr, err
}

// modifyArcNetworkDropRule blocks all network traffic from ARC interfaces.
// The caller of this function is required to tear down the updated state.
func modifyArcNetworkDropRule(ctx context.Context, op string) []error {
	var errs []error
	for _, cmd := range []string{"iptables", "ip6tables"} {
		if err := testexec.CommandContext(ctx, cmd, op, "FORWARD", "-t", "filter", "-i", "arc+", "-j", "DROP", "-w").Run(testexec.DumpLogOnError); err != nil {
			errs = append(errs, errors.Wrap(err, "failed to modify ARC traffic block rule"))
		}
	}
	return errs
}
