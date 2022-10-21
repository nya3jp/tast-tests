// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         StackSampledMetrics,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Check that stack-sampled metrics work",
		Contacts: []string{
			"iby@chromium.org",
			"cros-telemetry@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "stack_sampled_metrics"},
		Params: []testing.Param{{
			Fixture: "chromeLoggedInWithStackSampledMetrics",
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			Fixture:           "lacrosWithStackSampledMetrics",
			ExtraSoftwareDeps: []string{"lacros"},
			Val:               browser.TypeLacros,
		}},
	})
}

func StackSampledMetrics(ctx context.Context, s *testing.State) {
	// Reserve a few seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(cleanupCtx, 10*time.Second)
	defer cancel()

	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	// Set up the browser, open a window.
	const url = chrome.NewTabURL
	conn, br, closeBrowser, err := browserfixt.SetUpWithURL(ctx, cr, s.Param().(browser.Type), url)
	if err != nil {
		s.Fatal("Failed to open the browser: ", err)
	}
	defer closeBrowser(cleanupCtx)
	defer conn.Close()

	// TODO(b/214117401): Right now, this is just a smoke test. Once we have a
	// debugging hook to get information about the stack profiler, use that
	// instead of just sleeping.

	// Ensure stack sampler has generated a few samples. The --start-stack-profiler=browser-test
	// should ensure it gathers samples approximates once a second.
	testing.Sleep(ctx, 5*time.Second)

	// Ensure browser is still up.
	if _, err := br.CurrentTabs(ctx); err != nil {
		s.Fatal("Failed to get tabs: ", err)
	}
}
