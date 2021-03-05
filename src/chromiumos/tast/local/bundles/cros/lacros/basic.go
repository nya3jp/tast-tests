// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package lacros tests lacros-chrome running on ChromeOS.
package lacros

import (
	"context"
	"time"

	"chromiumos/tast/local/lacros/faillog"
	"chromiumos/tast/local/lacros/launcher"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Basic,
		Desc:         "Tests basic lacros startup",
		Contacts:     []string{"erikchen@chromium.org", "hidehiko@chromium.org", "edcourtney@chromium.org", "lacros-team@google.com"},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Fixture:      "lacrosStartedByData",
		Timeout:      7 * time.Minute,
		Data:         []string{launcher.DataArtifact},
	})
}

func Basic(ctx context.Context, s *testing.State) {
	l, err := launcher.LaunchLacrosChrome(ctx, s.FixtValue().(launcher.FixtData), s.DataPath(launcher.DataArtifact))
	if err != nil {
		s.Fatal("Failed to launch lacros-chrome: ", err)
	}
	defer func() {
		l.Close(ctx)
		if err := faillog.Save(s.HasError, l, s.OutDir()); err != nil {
			s.Log("Failed to save lacros logs: ", err)
		}
	}()

	if _, err = l.Devsess.CreateTarget(ctx, "about:blank"); err != nil {
		s.Fatal("Failed to open new tab: ", err)
	}
}
