// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package lacros tests linux-chrome running on ChromeOS.
package lacros

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/lacros/launcher"
	"chromiumos/tast/local/chrome"
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

func closeAboutBlank(ctx context.Context, ds *cdputil.Session) error {
	targetFilter := func(t *cdputil.Target) bool {
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
	if err = closeAboutBlank(ctx, l.Devsess); err != nil {
		s.Fatal("Failed to close about:blank tab: ", err)
	}
}
