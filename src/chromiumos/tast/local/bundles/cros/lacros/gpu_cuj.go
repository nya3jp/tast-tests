// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package lacros tests linux-chrome running on ChromeOS.
package lacros

import (
	"context"
	"time"

	"github.com/mafredri/cdp/protocol/target"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/lacros/launcher"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         GpuCUJ,
		Desc:         "Lacros GPU performance CUJ tests",
		Contacts:     []string{"edcourtney@chromium.org", "hidehiko@chromium.org", "lacros-team@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"disabled"},
		Pre:          launcher.StartedByData(),
		Timeout:      60 * time.Minute,
		Data:         []string{launcher.DataArtifact},
		Params: []testing.Param{{
			Name: "aquarium",
			Val:  "https://webglsamples.org/aquarium/aquarium.html",
		}, {
			Name: "poster",
			Val:  "https://webkit.org/blog-files/3d-transforms/poster-circle.html",
		}},
	})
}

func findFirstWindow(ctx context.Context, ctconn *chrome.TestConn) (*ash.Window, error) {
	return ash.FindWindow(ctx, ctconn, func(w *ash.Window) bool {
		return true
	})
}

func maximizeFirstWindow(ctx context.Context, ctconn *chrome.TestConn) error {
	w, err := findFirstWindow(ctx, ctconn)
	if err != nil {
		return err
	}
	_, err = ash.SetWindowState(ctx, ctconn, w.ID, ash.WMEventMaximize)
	return err
}

func closeAboutBlankForLacros(ctx context.Context, ds *cdputil.Session) error {
	targetFilter := func(t *target.Info) bool {
		return t.URL == chrome.BlankURL
	}
	targets, err := ds.FindTargets(ctx, targetFilter)
	if err != nil {
		return errors.Wrap(err, "failed to query for about:blank pages")
	}
	for _, info := range targets {
		ds.CloseTarget(ctx, info.TargetID)
	}
	return nil
}

func GpuCUJ(ctx context.Context, s *testing.State) {
	ctconn, err := s.PreValue().(launcher.PreData).Chrome.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	// Launch linux-chrome with about:blank loaded first - we don't want to include startup cost.
	l, err := launcher.LaunchLinuxChrome(ctx, s.PreValue().(launcher.PreData))
	if err != nil {
		s.Fatal("Failed to launch linux-chrome: ", err)
	}
	defer l.Close(ctx)

	// Wait for quiescent state.
	if err := cpu.WaitUntilIdle(ctx); err != nil {
		s.Fatal("Failed waiting for CPU to become idle: ", err)
	}

	conn, err := l.NewConn(ctx, s.Param().(string))
	if err != nil {
		s.Fatal("Failed to open new tab: ", err)
	}
	defer conn.Close()

	// Close the initial "about:blank" tab present at startup.
	err = closeAboutBlankForLacros(ctx, l.Devsess)
	if err != nil {
		s.Fatal("Failed to close about:blank tab: ", err)
	}

	// Maximize linux-chrome window.
	err = maximizeFirstWindow(ctx, ctconn)
	if err != nil {
		s.Fatal("Failed to maximize linux-chrome: ", err)
	}
}
