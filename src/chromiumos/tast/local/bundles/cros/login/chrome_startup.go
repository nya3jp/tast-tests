// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package login contains local Tast tests that exercise login scenarios on ChromeOS.
package login

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromeStartup,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks that Chrome can launch after login",
		Contacts:     []string{"zork@chromium.org", "chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val: browser.TypeAsh,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Val:               browser.TypeLacros,
		}},
		Timeout: time.Minute,
	})
}

func ChromeStartup(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	conn, err := cr.NewConn(ctx, "google.com")
	if err != nil {
		s.Fatal("Failed to navigate to website: ", err)
	}
	defer conn.Close()
}
