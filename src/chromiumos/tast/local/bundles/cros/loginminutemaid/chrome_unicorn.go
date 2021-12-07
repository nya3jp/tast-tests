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
			"unicorn.childUser",
			"unicorn.childPassword",
			"unicorn.parentUser",
			"unicorn.parentPassword",
		},
		Timeout: chrome.GAIALoginTimeout + time.Minute,
	})
}

func ChromeUnicorn(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(
		ctx,
		chrome.GAIALogin(chrome.Creds{
			User:       s.RequiredVar("unicorn.childUser"),
			Pass:       s.RequiredVar("unicorn.childPassword"),
			ParentUser: s.RequiredVar("unicorn.parentUser"),
			ParentPass: s.RequiredVar("unicorn.parentPassword"),
		}),
	)
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)
}
