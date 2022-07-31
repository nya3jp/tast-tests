// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package assistant

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/assistant"
	"chromiumos/tast/local/bundles/cros/assistant/assistantutils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/cpu"
	uiconsts "chromiumos/tast/local/ui"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type testParam struct {
	accel assistant.Accelerator
	bt    browser.Type
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         CardElementAnimationPerf,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Measures animation smoothness of card elements and transition from peeking to half height",
		Contacts:     []string{"cowmoo@chromium.org", "xiaohuic@chromium.org"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Params: []testing.Param{{
			Name:              "assistant_key",
			Val:               testParam{accel: assistant.AccelAssistantKey, bt: browser.TypeAsh},
			ExtraHardwareDeps: hwdep.D(hwdep.AssistantKey()),
			Fixture:           "assistantPerf",
		}, {
			Name:              "search_plus_a",
			Val:               testParam{accel: assistant.AccelSearchPlusA, bt: browser.TypeAsh},
			ExtraHardwareDeps: hwdep.D(hwdep.NoAssistantKey()),
			Fixture:           "assistantPerf",
		}, {
			Name:              "assistant_key_lacros",
			Val:               testParam{accel: assistant.AccelAssistantKey, bt: browser.TypeLacros},
			ExtraSoftwareDeps: []string{"lacros"},
			ExtraHardwareDeps: hwdep.D(hwdep.AssistantKey()),
			Fixture:           "assistantLacrosPerf",
		}, {
			Name:              "search_plus_a_lacros",
			Val:               testParam{accel: assistant.AccelSearchPlusA, bt: browser.TypeLacros},
			ExtraSoftwareDeps: []string{"lacros"},
			ExtraHardwareDeps: hwdep.D(hwdep.NoAssistantKey()),
			Fixture:           "assistantLacrosPerf",
		}},
	})
}

// CardElementAnimationPerf measures the animation performance of
// animating card elements in and out of the assistant frame. It also measures
// the performance of expanding the launcher from peeking to half height when
// a card is displayed.
func CardElementAnimationPerf(ctx context.Context, s *testing.State) {
	// Reserve a few seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(cleanupCtx, 10*time.Second)
	defer cancel()

	accel := s.Param().(testParam).accel
	bt := s.Param().(testParam).bt

	fixtData := s.FixtValue().(*assistant.FixtData)
	cr := fixtData.Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	if err := assistant.SetBetterOnboardingEnabled(ctx, tconn, false); err != nil {
		s.Fatal("Failed to disable better onboarding: ", err)
	}

	ui := uiauto.New(tconn)
	pv := perf.NewValues()
	var br *browser.Browser
	var closeBrowser func(ctx context.Context)
	for nWindows := 0; nWindows < 3; nWindows++ {
		// Open the browser windows. Also set up the browser the first time it opens.
		if nWindows == 0 {
			var conn *chrome.Conn
			var err error
			conn, br, closeBrowser, err = browserfixt.SetUpWithURL(ctx, cr, bt, uiconsts.PerftestURL)
			if err != nil {
				s.Fatal("Failed to create new browser window: ", err)
			}
			conn.Close()
			defer closeBrowser(cleanupCtx)
		} else {
			if err := ash.CreateWindows(ctx, tconn, br, uiconsts.PerftestURL, 1); err != nil {
				s.Fatal("Failed to create a new browser window: ", err)
			}
		}

		if err := cpu.WaitUntilIdle(ctx); err != nil {
			s.Error("Failed to wait for system cpu idle: ", err)
		}

		if err := assistant.ToggleUIWithHotkey(ctx, tconn, accel); err != nil {
			s.Fatal("Failed opening assistant: ", err)
		}

		histograms, err := metrics.RunAndWaitAll(
			ctx,
			tconn,
			time.Second,
			func(ctx context.Context) error {
				return runCardQueries(ctx, tconn, ui)
			},
			// Card element opacity fade in / out.
			"Ash.Assistant.AnimationSmoothness.CardElement",
		)
		if err != nil {
			s.Fatal("Failed to collect histograms: ", err)
		}
		if err := assistantutils.ProcessHistogram(histograms, pv, nWindows); err != nil {
			s.Fatal("Failed to process histograms: ", err)
		}
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Fatal("Failed saving perf data: ", err)
	}
}

// runCardQueries performs two card queries in order to test the animate in
// and animate out behavior of the first card.
func runCardQueries(ctx context.Context, tconn *chrome.TestConn, ui *uiauto.Context) error {
	for _, q := range []string{"Mount Everest", "Weather"} {
		if err := runCardQuery(ctx, tconn, ui, q); err != nil {
			return err
		}
	}

	return nil
}

// runCardQuery is a helper function for running an Assistant query and waiting
// for a card result.
func runCardQuery(ctx context.Context, tconn *chrome.TestConn, ui *uiauto.Context, query string) error {
	if _, err := assistant.SendTextQuery(ctx, tconn, query); err != nil {
		return errors.Wrapf(err, "could not send query: %s", query)
	}

	if err := ui.WithTimeout(5 * time.Second).WaitUntilExists(nodewith.ClassName("AssistantCardElementView"))(ctx); err != nil {
		return errors.Wrapf(err, "query results not shown for query %s within timeout", query)
	}

	return nil
}
