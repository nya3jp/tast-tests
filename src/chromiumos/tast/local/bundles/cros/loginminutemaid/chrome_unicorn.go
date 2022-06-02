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
		Func: ChromeUnicorn,
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
			"family.childEmail",
			"family.childPassword",
			"family.parentEmail",
			"family.parentPassword",
		},
		Timeout: chrome.GAIALoginTimeout + time.Minute,
	})
}

func ChromeUnicorn(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(
		ctx,
		chrome.GAIALogin(chrome.Creds{
			User:       s.RequiredVar("family.childEmail"),
			Pass:       s.RequiredVar("family.childPassword"),
			ParentUser: s.RequiredVar("family.parentEmail"),
			ParentPass: s.RequiredVar("family.parentPassword"),
		}),
	)
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)
}
