// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package unicorn is used for writing Unicorn tests.
package unicorn

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/familylink"
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
		Fixture:      "familyLinkUnicornLogin",
	})
}

func Login(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*familylink.FixtData).Chrome
	tconn := s.FixtValue().(*familylink.FixtData).TestConn

	if cr == nil {
		s.Fatal("Failed to start Chrome")
	}
	if tconn == nil {
		s.Fatal("Failed to create test API connection")
	}
}
