// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"regexp"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromePolicyPageStatusTimestamps,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Tests timestamps in status boxes on chrome://policy page",
		Contacts: []string{
			"sergiyb@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
		Params: []testing.Param{{
			Name:    "ash",
			Fixture: fixture.ChromePolicyLoggedIn,
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			ExtraAttr:         []string{"informational"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val:               browser.TypeLacros,
		}},
	})
}

// reloadPolicies clicks the "Reload policies" button on the chrome://policy
// page to reload policies. Although we could use `policyutil.Refresh`, we
// prefer clicking the button to ensure that it works as expected.
func reloadPolicies(ctx context.Context, conn *browser.Conn, s *testing.State) {
	if err := conn.Eval(ctx, `document.getElementById('reload-policies').click()`, nil); err != nil {
		s.Fatal("Failed to click Reload policies button: ", err)
	}

	// Wait until reload button becomes enabled again, i.e. policies reloaded.
	if err := conn.WaitForExpr(ctx, `!document.getElementById('reload-policies').disabled`); err != nil {
		s.Fatal("Failed while waiting for Reload policies button to become enabled again: ", err)
	}

	// TODO(crbug/1326565): Wait for policies to be reloaded.
	if s.Param().(browser.Type) == browser.TypeLacros {
		if err := testing.Sleep(ctx, 5*time.Second); err != nil {
			s.Fatal("Failed while waiting for policies to be reloaded: ", err)
		}
		if err := conn.Navigate(ctx, "chrome://policy"); err != nil {
			s.Fatal("Failed to refresh chrome://policy after reloading policies: ", err)
		}
	}

}

// Content from status boxes on chrome://policy keyed by the box name, e.g.
// "User policies". Each value is also a map from a field class name, e.g.
// "time-since-last-fetch-attempt", to a field value, e.g. "0 secs ago".
type statusBoxesMap map[string]map[string]string

func readStatusBoxes(ctx context.Context, conn *browser.Conn, s *testing.State) statusBoxesMap {
	if err := conn.WaitForExpr(ctx, `!document.getElementById('status-section').hidden`); err != nil {
		s.Fatal("Failed while waiting for status box to appear: ", err)
	}

	var boxes statusBoxesMap
	if err := conn.Eval(ctx, `(async() => {
		const statusSection = document.getElementById('status-section');
		const policies = getPolicyFieldsets();
		const statuses = {};
		for (let i = 0; i < policies.length; ++i) {
			const legend = policies[i].querySelector('legend').textContent;
			const entries = {};
			const rows = policies[i]
				.querySelectorAll('.status-entry div:nth-child(2)');
			for (let j = 0; j < rows.length; ++j) {
				entries[rows[j].className] = rows[j].textContent.trim();
			}
			statuses[legend.trim()] = entries;
		}
		return statuses;
	})()`, &boxes); err != nil {
		s.Fatal("Failed to read status boxes: ", err)
	}
	return boxes
}

func checkTime(boxes statusBoxesMap, boxNames []string, key string, matcher *regexp.Regexp, s *testing.State) {
	for _, boxName := range boxNames {
		status, ok := boxes[boxName]
		if !ok {
			s.Errorf("No status box named %s", boxName)
			return
		}

		if didMatch := matcher.Match([]byte(status[key])); !didMatch {
			s.Errorf("%s is invalid: %s (does not match `%s`)", key, status[key], matcher)
		}
	}
}

func ChromePolicyPageStatusTimestamps(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Setup browser based on the chrome type.
	br, closeBrowser, err := browserfixt.SetUp(ctx, cr, s.Param().(browser.Type))
	if err != nil {
		s.Fatal("Failed to open the browser: ", err)
	}
	defer closeBrowser(cleanupCtx)

	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree")

	// Select types of status boxes we want to check.
	boxNames := []string{"User policies"}
	if s.Param().(browser.Type) == browser.TypeAsh {
		// TODO(b/230109898): Add testing for device policies on Ash.
	}

	// Run actual test. Start by opening chrome://policy page.
	conn, err := br.NewConn(ctx, "chrome://policy")
	if err != nil {
		s.Fatal("Failed to load chrome://policy: ", err)
	}
	defer conn.Close()

	// This time regexp allows for up to 30 seconds more time to pass than
	// its name suggests. This helps reduce flakiness on slow devices.
	var zeroSecsAgoRE = regexp.MustCompile(`([0-9]|1[0-9]|2[0-9]|30) secs? ago`)
	var oneMinAgoRE = regexp.MustCompile(`1 min ago`)

	// Reload policies and immediately check that timestamps are at 0 secs ago.
	reloadPolicies(ctx, conn, s)
	newBoxes := readStatusBoxes(ctx, conn, s)
	checkTime(newBoxes, boxNames, "time-since-last-refresh", zeroSecsAgoRE, s)
	checkTime(newBoxes, boxNames, "time-since-last-fetch-attempt", zeroSecsAgoRE, s)

	// Sleep for 1 minute, refresh page, check that timestamps are updated.
	if err = testing.Sleep(ctx, time.Minute); err != nil {
		s.Fatal("Failed to sleep for 1 minute: ", err)
	}
	if err = conn.Navigate(ctx, "chrome://policy"); err != nil {
		s.Fatal("Failed to reload chrome://policy: ", err)
	}
	sleepyBoxes := readStatusBoxes(ctx, conn, s)
	checkTime(sleepyBoxes, boxNames, "time-since-last-refresh", oneMinAgoRE, s)
	checkTime(sleepyBoxes, boxNames, "time-since-last-fetch-attempt", oneMinAgoRE, s)

	// Simulate 500 error on the server while reloading policies, check that fetch
	// timestamp is at 0 secs ago whilst policy timestamp is still at 1 min ago.
	pb := policy.NewBlob()
	pb.RequestErrors = make(map[string]int)
	pb.RequestErrors["policy"] = 500
	if err = policyutil.ServeBlobAndRefresh(ctx, fdms, cr, pb); err != nil {
		s.Fatal("Failed to simulate error 500 on the policy server: ", err)
	}
	reloadPolicies(ctx, conn, s)
	mixedBoxes := readStatusBoxes(ctx, conn, s)
	checkTime(mixedBoxes, boxNames, "time-since-last-refresh", oneMinAgoRE, s)
	checkTime(mixedBoxes, boxNames, "time-since-last-fetch-attempt", zeroSecsAgoRE, s)
}
