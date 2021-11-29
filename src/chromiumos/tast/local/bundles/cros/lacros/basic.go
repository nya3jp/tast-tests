// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package lacros tests lacros-chrome running on ChromeOS.
package lacros

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/lacros/lacrosfaillog"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Basic,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Tests basic lacros startup",
		Contacts:     []string{"erikchen@chromium.org", "hidehiko@chromium.org", "edcourtney@chromium.org", "lacros-team@google.com", "chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Fixture:      "lacros",
		Timeout:      7 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"lacros_stable"},
		}, {
			Name:              "unstable",
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"lacros_unstable"},
		}},
	})
}

func Basic(ctx context.Context, s *testing.State) {
	l, err := lacros.Launch(ctx, s.FixtValue().(lacrosfixt.FixtValue))
	defer lacrosfaillog.SaveIf(ctx, s.FixtValue().(lacrosfixt.FixtValue).LacrosPath(), s.HasError)

	if err != nil {
		s.Fatal("Failed to launch lacros-chrome: ", err)
	}
	defer l.Close(ctx)

	// Test that a new blank tab can be opened.
	conn, err := l.NewConn(ctx, "about:blank")
	if err != nil {
		s.Fatal("Failed to open new tab: ", err)
	}
	if err := conn.Close(); err != nil {
		s.Error("Failed to close connection: ", err)
	}
}
