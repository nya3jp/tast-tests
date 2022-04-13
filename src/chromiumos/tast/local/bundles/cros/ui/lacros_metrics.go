// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/graphics"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LacrosMetrics,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Test metrics collection from lacros-Chrome",
		Contacts:     []string{"xliu@cienet.com"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Fixture:      "lacrosPrimary",
	})
}

func LacrosMetrics(ctx context.Context, s *testing.State) {
	f := s.FixtValue().(lacrosfixt.FixtValue)
	cr := f.Chrome()
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
	defer faillog.SaveScreenshotOnError(ctx, cr, s.OutDir(), s.HasError)

	_, l, _, err := lacros.Setup(ctx, f, browser.TypeLacros)
	if err != nil {
		s.Fatal("Failed to set up lacros test: ", err)
	}
	// defer lacros.CloseLacros(ctx, l)
	bTconn, err := l.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to lacros test API: ", err)
	}
	tconns := []*chrome.TestConn{tconn, bTconn}

	ui := uiauto.New(tconn)

	recorder, err := cuj.NewRecorder(ctx, cr, nil, cuj.MetricConfigs(tconns)...)
	if err != nil {
		s.Fatal("Failed to create the recorder: ", err)
	}
	defer recorder.Close(ctx)

	if err := recorder.Run(ctx, func(ctx context.Context) error {
		testing.ContextLog(ctx, "Navigate to google.com")
		_, err = l.NewConn(ctx, "https://www.google.com")
		if err != nil {
			s.Fatal("Failed to open new tab: ", err)
		}

		searchBox := nodewith.Name("Search").Role(role.TextFieldWithComboBox).Focused()
		testing.ContextLog(ctx, "Click search box")
		if err := ui.LeftClick(searchBox)(ctx); err != nil {
			s.Fatal("Failed to click search box: ", err)
		}

		kb, err := input.Keyboard(ctx)
		if err != nil {
			s.Fatal("Failed to find keyboard: ", err)
		}
		defer kb.Close()
		testing.ContextLog(ctx, "Type text in search box")
		if err := kb.Type(ctx, "Hello"); err != nil {
			s.Fatal("Failed to enter search text Hello: ", err)
		}

		if err := kb.AccelAction("Enter")(ctx); err != nil {
			s.Fatal("Failed to typer enter to start search: ", err)
		}

		testing.Sleep(ctx, 5*time.Second)
		return nil
	}); err != nil {
		s.Fatal("Failed to conduct the test: ", err)
	}

	s.Log("Getting ready for the next recorder run")
	testing.Sleep(ctx, 5*time.Second)

	pv := perf.NewValues()
	if err := recorder.Run(ctx, func(ctx context.Context) error {
		errc := make(chan error)
		go func() {
			// Using goroutine to measure GPU counters asynchronously because:
			// - we will add some other test scenarios (controlling windows / meet sessions).
			// - graphics.MeasureGPUCounters may quit immediately when the hardware or
			//   kernel does not support the reporting mechanism.
			errc <- graphics.MeasureGPUCounters(ctx, 5*time.Second, pv)
		}()

		testing.ContextLog(ctx, "Navigate to google.com in a new lacros browser window")
		_, err = l.NewConn(ctx, "https://www.google.com", browser.WithNewWindow())
		if err != nil {
			s.Fatal("Failed to open new tab: ", err)
		}

		searchBox := nodewith.Name("Search").Role(role.TextFieldWithComboBox).Focused()
		testing.ContextLog(ctx, "Click search box")
		if err := ui.LeftClick(searchBox)(ctx); err != nil {
			s.Fatal("Failed to click search box: ", err)
		}

		kb, err := input.Keyboard(ctx)
		if err != nil {
			s.Fatal("Failed to find keyboard: ", err)
		}
		defer kb.Close()
		testing.ContextLog(ctx, "Type text in search box")
		if err := kb.Type(ctx, "Hello"); err != nil {
			s.Fatal("Failed to enter search text Hello: ", err)
		}

		if err := kb.AccelAction("Enter")(ctx); err != nil {
			s.Fatal("Failed to typer enter to start search: ", err)
		}

		testing.Sleep(ctx, 5*time.Second)

		if err := <-errc; err != nil {
			return errors.Wrap(err, "failed to collect GPU counters")
		}
		return nil
	}); err != nil {
		s.Fatal("Failed to conduct the test: ", err)
	}

	if err := recorder.Record(ctx, pv); err != nil {
		s.Fatal("Failed to record the performance data: ", err)
	}
	if err := pv.Save(s.OutDir()); err != nil {
		s.Fatal("Failed to save the performance data: ", err)
	}
	if err := recorder.SaveHistograms(s.OutDir()); err != nil {
		s.Fatal("Failed to save histogram raw data: ", err)
	}
}
