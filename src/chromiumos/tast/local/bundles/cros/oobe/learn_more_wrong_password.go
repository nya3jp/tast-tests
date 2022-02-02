// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package oobe

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/lockscreen"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/session"
	"chromiumos/tast/testing"
)

const (
	numTries = 2
)

func init() {
	testing.AddTest(&testing.Test{
		Func: LearnMoreWrongPassword,
		Desc: "Checks that there is a \"Learn More\" link after the user enters wrong password twice",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
			"cros-oac@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		VarDeps:      []string{"ui.signinProfileTestExtensionManifestKey"},
	})
}

// LearnMoreWrongPassword logs out, enter wrong password twice and verifies that Learn More link is shown.
func LearnMoreWrongPassword(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	const (
		username = "testuser@gmail.com"
		password = "testpass"
	)

	cr, err := chrome.New(ctx, chrome.FakeLogin(chrome.Creds{User: username, Pass: password}))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get test API connection")
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to initialize keyboard: ", err)
	}
	defer kb.Close()

	sm, err := session.NewSessionManager(ctx)
	if err != nil {
		s.Fatal("Failed to connect to session manager: ", err)
	}

	const state = "stopped"
	sw, err := sm.WatchSessionStateChanged(ctx, state)
	if err != nil {
		s.Fatal("Failed to watch for D-Bus signals: ", err)
	}
	defer sw.Close(ctx)

	if err := quicksettings.SignOut(ctx, tconn); err != nil {
		s.Fatal("Failed to logout: ", err)
	}

	s.Logf("Waiting for SessionStateChanged %q D-Bus signal from session_manager", state)
	select {
	case <-sw.Signals:
		s.Log("Got SessionStateChanged signal")
	case <-ctx.Done():
		s.Fatal("Didn't get SessionStateChanged signal: ", ctx.Err())
	}

	if cr, err = chrome.New(ctx,
		chrome.ExtraArgs("--skip-force-online-signin-for-testing"),
		chrome.NoLogin(),
		chrome.KeepState(),
		chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")),
	); err != nil {
		s.Fatal("Failed to restart Chrome for testing: ", err)
	}
	defer cr.Close(cleanupCtx)

	if tconn, err = cr.SigninProfileTestAPIConn(ctx); err != nil {
		s.Fatal("Failed to re-establish test API connection")
	}

	if _, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return st.ReadyForPassword }, 10*time.Second); err != nil {
		s.Fatal("Failed to wait for login screen: ", err)
	}

	wrongPassword := password + "1"
	for i := 0; i < numTries; i++ {
		if err := lockscreen.EnterPassword(ctx, tconn, username, wrongPassword, kb); err != nil {
			s.Fatal("Failed to enter password: ", err)
		}
		time.Sleep(time.Second * 2)
	}
}
