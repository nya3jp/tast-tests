// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ChromeLoginGAIA,
		Desc: "Checks that Chrome can make real GAIA logins",
		Contacts: []string{
			"chromeos-ui@google.com",
			"tast-owners@google.com",
		},
		SoftwareDeps: []string{
			"chrome",
			"chrome_internal",
		},
		Attr: []string{
			"group:mainline",
		},
		Vars: []string{
			"ui.gaiaPoolDefault",
		},
		Timeout: chrome.GAIALoginTimeout + time.Minute,
	})
}

func ChromeLoginGAIA(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(
		ctx,
		chrome.AuthPool(s.RequiredVar("ui.gaiaPoolDefault")),
		chrome.GAIALogin(),
	)
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)
}
