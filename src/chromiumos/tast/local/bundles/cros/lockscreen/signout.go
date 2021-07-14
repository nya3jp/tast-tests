// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lockscreen

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/chromeproc"
	"chromiumos/tast/local/chrome/ui/lockscreen"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Signout,
		Desc:         "Test signout from the lock screen",
		Contacts:     []string{"chromeos-sw-engprod@google.com", "cros-oac@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
	})
}

func Signout(ctx context.Context, s *testing.State) {
	const (
		username = "testuser@gmail.com"
		password = "good"
		PIN      = "1234567890"
	)

	cr, err := chrome.New(ctx, chrome.FakeLogin(chrome.Creds{User: username, Pass: password}))
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Getting test API connection failed: ", err)
	}

	cr.NewConn(ctx, "http://www.example.org/")
	cr.NewConn(ctx, "http://youtube.com/")
	cr.NewConn(ctx, "http://maps.google.com/")

	// Lock the screen.
	if err := lockscreen.Lock(ctx, tconn); err != nil {
		s.Fatal("Failed to lock the screen: ", err)
	}

	if err := chrome.PrepareForRestart(); err != nil {
		s.Fatal("Failed to prepare to restart: ", err)
	}

	oldpid, err := chromeproc.GetRootPID()
	if err != nil {
		s.Fatal("Failed to get Chrome root PID: ", err)
	}
	testing.ContextLogf(ctx, "oldpid %d", oldpid)

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
	ui := uiauto.New(tconn)
	if err := ui.LeftClick(nodewith.Name("Sign out").Role(role.Button))(ctx); err != nil {
		s.Log("Failed to click on Signout button: ", err)
	}
	// What to check here?

	newpid, err := chromeproc.GetRootPID()
	if err != nil {
		s.Fatal("Failed to get Chrome root PID: ", err)
	}
	testing.ContextLogf(ctx, "newpid %d", newpid)

	if cr, err := chrome.New(ctx, chrome.NoLogin(), chrome.TryReuseSession()); err != nil {
		s.Fatal("Could not reuse login screen: ", err)
		cr.NewConn(ctx, "http://www.example.org/")
	}

	testing.ContextLog(ctx, "locked")
	if st, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return st.LoggedIn && st.ReadyForPassword }, 30*time.Second); err != nil {
		s.Fatalf("Waiting for screen to be locked failed: %v (last status %+v)", err, st)
	}
}
