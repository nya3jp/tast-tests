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

func init() {
	testing.AddTest(&testing.Test{
		Func:     UIauto,
		Desc:     "UIauto",
		Contacts: []string{"yamaguchi@chromium.org", "tast-owners@google.com"},
		Attr:     []string{},
		Fixture:  "chromeLoggedIn",
		Params: []testing.Param{{
			Name:      "mainline",
			ExtraAttr: []string{"group:mainline", "informational"},
		}, {}},
	})
}

func UIauto(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	ui := uiauto.New(tconn)
	s.Log("is mainline: ", ctx.Value(testing.IsMainline{}))
	s.Log("exists with role: ", ui.Exists(nodewith.Role("A"))(ctx))
	s.Log("exists with name: ", ui.Exists(nodewith.Name("A"))(ctx))
}
