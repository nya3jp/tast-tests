// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/testing"
)

type testConf struct {
	useName bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:     UIauto,
		Desc:     "UIauto",
		Contacts: []string{"yamaguchi@chromium.org", "tast-owners@google.com"},
		Attr:     []string{},
		Params: []testing.Param{{
			// fail
			Name:      "mainline",
			ExtraAttr: []string{"group:mainline", "informational"},
			Fixture:   "chromeLoggedIn",
			Val:       testConf{true},
			// error; cannot be used with mainline
			// If uncommented, Tast will fail while registering tests before running any test.
			// ExtraLabel: []string{"allow_fragile"}
		}, {
			// pass
			Name:       "allow",
			Fixture:    "chromeLoggedIn",
			Val:        testConf{true},
			ExtraLabel: []string{"allow_fragile"},
		}, {
			// pass
			Name:    "nouse_name",
			Fixture: "chromeLoggedIn",
			Val:     testConf{false},
		}, {
			// fail
			Name:    "fixture",
			Fixture: "networkDiagnostics", // uses Name() matcher inside
			Val:     testConf{false},
		}},
	})
}

func UIauto(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	ui := uiauto.New(tconn)
	if _, err := ui.IsNodeFound(ctx, nodewith.Role("A")); err != nil {
		s.Error("nodewith.Role error: ", err)
	}
	x, _ := s.Param().(testConf)
	if x.useName {
		if _, err := ui.IsNodeFound(ctx, nodewith.Name("A")); err != nil {
			s.Error("nodewith.Name error: ", err)
		}
	}
}
