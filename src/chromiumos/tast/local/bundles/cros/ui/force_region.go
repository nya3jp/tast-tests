// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ForceRegion,
		Desc:         "Checks that region is forced in Chrome tests",
		Contacts:     []string{"nya@chromium.org", "chromeos-ui@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"informational"},
	})
}

func ForceRegion(ctx context.Context, s *testing.State) {
	const (
		region   = "jp"
		wantLang = "ja"
	)

	cr, err := chrome.New(ctx, chrome.Region(region))
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	conn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	var lang string
	if err := conn.Eval(ctx, "chrome.i18n.getUILanguage()", &lang); err != nil {
		s.Fatal("Failed to call chrome.i18n.getUILanguage: ", err)
	} else if lang != wantLang {
		s.Fatalf("UI language is %s; want %s", lang, wantLang)
	}
}
