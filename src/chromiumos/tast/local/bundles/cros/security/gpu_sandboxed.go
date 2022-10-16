// Copyright 2018 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         GPUSandboxed,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Verify GPU sandbox status",
		Contacts: []string{
			"jorgelo@chromium.org",  // Security team
			"hidehiko@chromium.org", // Tast port author
			"chromeos-security@google.com",
		},
		SoftwareDeps: []string{"chrome", "gpu_sandboxing"},
		Attr:         []string{"group:mainline"},
		Params: []testing.Param{{
			Fixture: "chromeLoggedIn",
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			Fixture:           "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Val:               browser.TypeLacros,
		}},
		Timeout: 1 * time.Minute,
	})
}

func GPUSandboxed(ctx context.Context, s *testing.State) {
	const (
		url      = "chrome://gpu/"
		waitExpr = "browserBridge.isSandboxedForTesting()"
	)

	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	conn, _, closeBrowser, err := browserfixt.SetUpWithURL(ctx, cr, s.Param().(browser.Type), url)
	if err != nil {
		s.Fatal("Failed to create a new connection: ", err)
	}
	defer closeBrowser(ctx)
	defer conn.Close()

	ectx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err = conn.WaitForExpr(ectx, waitExpr); err != nil {
		s.Fatalf("Failed to evaluate %q in %s", waitExpr, url)
	}
}
