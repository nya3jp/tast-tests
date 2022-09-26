// Copyright 2020 The ChromiumOS Authors
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
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Disable3DAPIs,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Behavior of the Disable3DAPIs policy",
		Contacts: []string{
			"vsavu@google.com", // Test author
			"kathrelkeld@chromium.org",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
		Params: []testing.Param{{
			Fixture: fixture.ChromePolicyLoggedIn,
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			ExtraAttr:         []string{"informational"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val:               browser.TypeLacros,
		}},
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.Disable3DAPIs{}, pci.VerifiedFunctionalityJS),
		},
	})
}

func Disable3DAPIs(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	for _, param := range []struct {
		// name is the subtest name.
		name string
		// value is the policy value.
		value *policy.Disable3DAPIs
	}{
		{
			name:  "unset",
			value: &policy.Disable3DAPIs{Stat: policy.StatusUnset},
		},
		{
			name:  "false",
			value: &policy.Disable3DAPIs{Val: false},
		},
		{
			name:  "true",
			value: &policy.Disable3DAPIs{Val: true},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Perform cleanup
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// TODO(crbug.com/1259615): This should be part of the fixture.
			br, closeBrowser, err := browserfixt.SetUp(ctx, cr, s.Param().(browser.Type))
			if err != nil {
				s.Fatal("Failed to setup chrome: ", err)
			}
			defer closeBrowser(cleanupCtx)

			// Run actual test.
			conn, err := br.NewConn(ctx, "")
			if err != nil {
				s.Fatal("Failed to connect to chrome: ", err)
			}
			defer conn.Close()

			var enabled bool
			if err := conn.Eval(ctx, `(() => {
			  try {
			    let canvas = document.createElement('canvas');
			    return !!canvas.getContext('webgl');
			  } catch (e) {
			    return false;
			  }
			})()`, &enabled); err != nil {
				s.Fatal("Could not get webgl status: ", err)
			}

			expectEnabled := param.value.Stat == policy.StatusUnset || param.value.Val == false

			if !expectEnabled && enabled {
				s.Error("WebGL not blocked")
			}

			// WebGL may not be available on all devices.
			if expectEnabled && !enabled {
				s.Log("WebGL not available")
			}
		})
	}
}
