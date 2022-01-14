// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const (
	videocallURL        = "https://storage.googleapis.com/chromiumos-test-assets-public/power_VideoCall/power_VideoCall.html"
	docsURL             = "http://crospower.page.link/power_VideoCall_doc"
	measurementDuration = 60 * time.Second
	notes               = "Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. "
)

type docsVCTest struct {
	useLacros bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         PowerVideocall,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Runs a video conference and Docs window side-by-side with both ash-crhome and lacros-chrome",
		Contacts:     []string{"luken@google.com", "hidehiko@chromium.org", "lacros-team@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		SoftwareDeps: []string{"chrome", "arc"},
		Timeout:      20 * time.Minute,
		Params: []testing.Param{{
			Name: "lacros",
			Val: docsVCTest{
				useLacros: true,
			},
			Fixture:           "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
		}, {
			Name: "ash",
			Val: docsVCTest{
				useLacros: false,
			},
			Fixture: "lacros",
		}},
	})
}

func PowerVideocall(ctx context.Context, s *testing.State) {
	docsVC := s.Param().(docsVCTest)
	cr := s.FixtValue().(lacrosfixt.FixtValue).Chrome()

	var cs ash.ConnSource
	var bTconn *chrome.TestConn

	if !docsVC.useLacros {
		cs = cr
		var err error
		bTconn, err = cr.TestAPIConn(ctx)
		if err != nil {
			s.Fatal("Failed to get TestAPIConn: ", err)
		}
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to the test API connection: ", err)
	}

	// If using Lacros we have an open window from the shelf launch, but no window object for it.
	// So we find the open window and save it so we can close it after opening the video
	// window.
	var extraLacrosConn *chrome.Conn
	if docsVC.useLacros {
		f := s.FixtValue().(lacrosfixt.FixtValue)

		l, err := lacros.LaunchFromShelf(ctx, tconn, f.LacrosPath())
		if err != nil {
			s.Fatal("Failed to launch lacros: ", err)
		}
		defer l.Close(ctx)
		cs = l

		if bTconn, err = l.TestAPIConn(ctx); err != nil {
			s.Fatal("Failed to get lacros TestAPIConn: ", err)
		}

		extraLacrosConn, err = l.NewConnForTarget(ctx, chrome.MatchTargetURL("chrome://newtab/"))
		if err != nil {
			s.Fatal("Failed to find lacros new window: ", err)
		}
	}

	// Create the VC window.
	videoConn, err := cs.NewConn(ctx, chrome.BlankURL, browser.WithNewWindow())
	if err != nil {
		s.Fatal("Failed to open VC window: ", err)
	}
	defer func() {
		videoConn.CloseTarget(ctx)
		videoConn.Close()
	}()

	if docsVC.useLacros {
		if err := extraLacrosConn.CloseTarget(ctx); err != nil {
			s.Fatal("Failed to close initial Lacros window: ", err)
		}
	}

	videoWin, err := lacros.FindFirstBlankWindow(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to find blank VC window: ", err)
	}

	if err := ash.SetWindowStateAndWait(ctx, tconn, videoWin.ID, ash.WindowStateRightSnapped); err != nil {
		s.Error("Failed to snap first blank window to the right: ", err)
	}

	if err := videoConn.Navigate(ctx, videocallURL); err != nil {
		s.Error("Failed to navigate first blank window to video call: ", err)
	}

	pc := pointer.NewMouse(tconn)

	bubble := nodewith.ClassName("PermissionPromptBubbleView").First()
	allow := nodewith.Name("Allow").Role(role.Button).Ancestor(bubble)
	// Check and grant permissions.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		needPermission, err := needToGrantPermission(ctx, videoConn)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to check if it needs to grant permissions"))
		}
		if !needPermission {
			return nil
		}
		if err := pc.Click(allow)(ctx); err != nil {
			return errors.Wrap(err, "failed to click the allow button")
		}
		return errors.New("granting permissions")
	}, &testing.PollOptions{Interval: time.Second, Timeout: 2 * time.Minute}); err != nil {
		s.Fatal("Failed to grant permissions: ", err)
	}

	// Create the docs window.
	docsConn, err := cs.NewConn(ctx, chrome.BlankURL, browser.WithNewWindow())
	if err != nil {
		s.Fatal("Failed to open docs window: ", err)
	}
	defer func() {
		docsConn.CloseTarget(ctx)
		docsConn.Close()
	}()

	docsWin, err := lacros.FindFirstBlankWindow(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to find blank docs window: ", err)
	}

	if err := ash.SetWindowStateAndWait(ctx, tconn, docsWin.ID, ash.WindowStateLeftSnapped); err != nil {
		s.Error("Failed to snap second blank window to the left: ", err)
	}

	if err := docsConn.Navigate(ctx, docsURL); err != nil {
		s.Error("Failed to navigate second blank window to docs page: ", err)
	}

	kw, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create a keyboard: ", err)
	}
	defer kw.Close()

	// Select text input field
	if err := uiauto.Combine(
		"short sleep than select text input field",
		action.Sleep(1*time.Second),
		pc.Click(nodewith.Name("Edit here").Role(role.TextField)))(ctx); err != nil {
		s.Fatal("Failed to select input field on docs page: ", err)
	}

	histograms, err := metrics.RunAndWaitAll(
		ctx,
		bTconn,
		2*measurementDuration,
		func(ctx context.Context) error {
			end := time.Now()
			for time.Now().Sub(end) < measurementDuration {
				if err := uiauto.Combine(
					"sleep and type",
					action.Sleep(5*time.Second),
					kw.TypeAction(notes),
				)(ctx); err != nil {
					return err
				}
			}
			return nil
		},
		"Event.Latency.EndToEnd.KeyPress",
	)
	if err != nil {
		s.Fatal("Failed to collect metric data: ", err)
	}

	pv := perf.NewValues()

	for _, h := range histograms {
		mean, err := h.Mean()
		if err != nil {
			s.Error("Failed to extract mean from histogram: ", err)
		}
		pv.Set(
			perf.Metric{
				Name:      fmt.Sprintf("%s.mean", h.Name),
				Unit:      "microseconds",
				Direction: perf.SmallerIsBetter,
			},
			mean,
		)
		totalCount := h.TotalCount()
		sampleNum95 := (totalCount * 95) / 100
		var value95 float64
		var t int64
		for _, b := range h.Buckets {
			if t < sampleNum95 {
				if t+b.Count >= sampleNum95 {
					value95 = float64(b.Min) + ((float64(b.Max) - float64(b.Min)) * (float64(sampleNum95) - float64(t)))
				}
			}
			t = t + b.Count
		}
		pv.Set(
			perf.Metric{
				Name:      fmt.Sprintf("%s.percent_95", h.Name),
				Unit:      "microseconds",
				Direction: perf.SmallerIsBetter,
			},
			value95,
		)
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to save the perf data: ", err)
	}
}

// needToGrantPermission checks if we need to grant permission before joining meetings.
// If camera/microphone permissions are not granted, we need to skip the permission bubbles later.
func needToGrantPermission(ctx context.Context, conn *chrome.Conn) (bool, error) {
	perms := []string{"microphone", "camera"}
	for _, perm := range perms {
		var state string
		if err := conn.Eval(ctx, fmt.Sprintf(
			`new Promise(function(resolve, reject) {
				navigator.permissions.query({name: '%v'})
				.then((permission) => {
					resolve(permission.state);
				})
				.catch((error) => {
					reject(error);
				});
			 })`, perm), &state); err != nil {
			return true, errors.Errorf("failed to query %v permission", perm)
		}
		if state != "granted" {
			return true, nil
		}
	}
	return false, nil
}
