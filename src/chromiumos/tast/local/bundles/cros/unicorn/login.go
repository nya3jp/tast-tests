// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package unicorn is used for writing Unicorn tests.
package unicorn

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Login,
		Desc:         "Checks if Unicorn login is working",
		Contacts:     []string{"chromeos-sw-engprod@google.com", "cros-oac@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Vars:         []string{"unicorn.parentUser", "unicorn.parentPassword", "unicorn.childUser", "unicorn.childPassword"},
		Timeout:      chrome.GAIALoginTimeout + time.Minute,
	})
}

func Login(ctx context.Context, s *testing.State) {
	parentUser := s.RequiredVar("unicorn.parentUser")
	parentPass := s.RequiredVar("unicorn.parentPassword")
	childUser := s.RequiredVar("unicorn.childUser")
	childPass := s.RequiredVar("unicorn.childPassword")

	cr, err := chrome.New(ctx,
		chrome.GAIALogin(chrome.Creds{
			User:       childUser,
			Pass:       childPass,
			ParentUser: parentUser,
			ParentPass: parentPass,
		}))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)
}
