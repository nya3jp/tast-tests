// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Optin,
		Desc: "A functional test that verifies OptIn flow",
		Contacts: []string{
			"arc-core@google.com",
			"khmel@chromium.org", // author.
		},
		// TODO(khmel): Make it critical.
		Attr: []string{"group:mainline"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p", "chrome"},
			Val:               []string{},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm", "chrome"},
			Val:               []string{"--enable-arcvm"},
		}},
		Timeout: 5 * time.Minute,
		Vars:    []string{"arc.Optin.username", "arc.Optin.password"},
	})
}

func Optin(ctx context.Context, s *testing.State) {
	username := s.RequiredVar("arc.Optin.username")
	password := s.RequiredVar("arc.Optin.password")

	// Setup Chrome.
	extraArgs := s.Param().([]string)
	args := []string{"--arc-disable-app-sync", "--arc-disable-play-auto-install", "--arc-disable-locale-sync", "--arc-play-store-auto-update=off"}
	args = append(args, extraArgs...)
	cr, err := chrome.New(ctx, chrome.GAIALogin(),
		chrome.Auth(username, password, "gaia-id"), chrome.ARCSupported(),
		chrome.ExtraArgs(args...))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	s.Log("Performing optin")
	if err := optin.Perform(ctx, cr, tconn); err != nil {
		s.Error("Failed to optin: ", err)
	}
}
