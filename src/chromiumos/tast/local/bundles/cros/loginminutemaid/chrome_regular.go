// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package loginminutemaid

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ChromeRegular,
		Desc: "Checks that Chrome can make real GAIA logins",
		Contacts: []string{
			"identity-testing-coreteam@google.com",
		},
		SoftwareDeps: []string{
			"chrome",
			"chrome_internal",
		},
		Attr: []string{},
		VarDeps: []string{
			"ui.gaiaPoolDefault",
		},
		Timeout: chrome.GAIALoginTimeout + time.Minute,
	})
}

func ChromeRegular(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(
		ctx,
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
	)
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)
}
