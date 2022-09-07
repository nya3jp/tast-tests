// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package familylink is used for writing Family Link tests.
package familylink

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         UnicornStartup,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks that Unicorn users work",
		Contacts:     []string{"zork@chromium.org", "cros-families-eng+test@google.com", "chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Vars: []string{
			"family.parentEmail",
			"family.parentPassword",
			"family.unicornEmail",
			"family.unicornPassword",
		},
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

func UnicornStartup(ctx context.Context, s *testing.State) {
	childUser := s.RequiredVar("family.unicornEmail")
	childPass := s.RequiredVar("family.unicornPassword")
	parentUser := s.RequiredVar("family.parentEmail")
	parentPass := s.RequiredVar("family.parentPassword")
	opts := []chrome.Option{
		chrome.ExtraArgs("--force-devtools-available"),
		chrome.GAIALogin(chrome.Creds{
			User:       childUser,
			Pass:       childPass,
			ParentUser: parentUser,
			ParentPass: parentPass,
		})}

	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	conn, err := cr.NewConn(ctx, "google.com")
	if err != nil {
		s.Fatal("Failed to navigate to website: ", err)
	}
	defer conn.Close()
}
