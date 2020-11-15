// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package lacros tests lacros-chrome running on ChromeOS.
package lacros

import (
	"context"
	"time"

	"chromiumos/tast/local/lacros/launcher"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Basic,
		Desc:         "Tests basic lacros startup",
		Contacts:     []string{"erikchen@chromium.org", "hidehiko@chromium.org", "edcourtney@chromium.org", "lacros-team@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Timeout:      7 * time.Minute,
		Params: []testing.Param{
			{
				Pre:       launcher.StartedByData(),
				ExtraData: []string{launcher.DataArtifact},
			},
			{
				Name:              "omaha",
				Pre:               launcher.StartedByFlag(),
				ExtraHardwareDeps: hwdep.D(hwdep.Model("enguarde", "samus", "sparky")),
			}},
	})
}

func Basic(ctx context.Context, s *testing.State) {
	l, err := launcher.LaunchLacrosChrome(ctx, s.PreValue().(launcher.PreData))
	if err != nil {
		s.Fatal("Failed to launch lacros-chrome: ", err)
	}
	defer l.Close(ctx)

	if _, err = l.Devsess.CreateTarget(ctx, "about:blank"); err != nil {
		s.Fatal("Failed to open new tab: ", err)
	}
}
