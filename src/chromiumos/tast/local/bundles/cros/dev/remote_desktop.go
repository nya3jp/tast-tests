// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dev

import (
	"context"
	"strconv"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto/crd"
	"chromiumos/tast/local/dev"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type testParam struct {
	browserType browser.Type
	manual      bool
}

func shouldWait(s *testing.State, manual bool) bool {
	waitStr, ok := s.Var("wait")
	if !ok {
		// Only wait for remote connection when running manually.
		if manual {
			waitStr = "true"
		} else {
			waitStr = "false"
		}
	}
	wait, err := strconv.ParseBool(waitStr)
	if err != nil {
		s.Fatal("Failed to parse the variable `wait`: ", err)
	}
	return wait
}

func init() {
	// Example usage:
	// $ tast run -var=user=<username> -var=pass=<password> <dut ip> dev.RemoteDesktop
	// <username> and <password> are the credentials of the test GAIA account.
	testing.AddTest(&testing.Test{
		Func:         RemoteDesktop,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Connect to Chrome Remote Desktop for working remotely",
		Contacts:     []string{"shik@chromium.org", "tast-users@chromium.org"},
		SoftwareDeps: []string{"chrome"},
		Vars: []string{
			// For running manually.
			"wait",
		},
		Params: []testing.Param{{
			// For running manually.
			Name:    "",
			Fixture: dev.ChromeLoggedIn,
			Val:     testParam{browserType: browser.TypeAsh, manual: true},
		}, {
			// For automated testing.
			Name:      "test",
			Fixture:   dev.ChromeLoggedIn,
			ExtraAttr: []string{"group:mainline", "informational"},
			// TODO(b/151111783): This is a speculative fix to limit the number of sessions. It
			// seems that the test account is throttled by the CRD backend, so the test is failing
			// with a periodic pattern. The model list is handcrafted to cover various platforms.
			ExtraHardwareDeps: hwdep.D(hwdep.Model("atlas", "careena", "dru", "eve", "kohaku",
				"krane", "nocturne")),
			Val: testParam{browserType: browser.TypeAsh, manual: false},
		}, {
			// For automated testing with lacros.
			Name:              "lacros_test",
			Fixture:           dev.LacrosLoggedIn,
			ExtraAttr:         []string{"group:mainline", "informational"},
			ExtraSoftwareDeps: []string{"lacros"},
			// TODO(b/151111783): This is a speculative fix to limit the number of sessions. It
			// seems that the test account is throttled by the CRD backend, so the test is failing
			// with a periodic pattern. The model list is handcrafted to cover various platforms.
			ExtraHardwareDeps: hwdep.D(hwdep.Model("atlas", "careena", "dru", "eve", "kohaku",
				"krane", "nocturne")),
			Val: testParam{browserType: browser.TypeLacros, manual: false},
		}},
	})
}

func RemoteDesktop(ctx context.Context, s *testing.State) {
	// TODO(shik): The button names only work in English locale, and adding
	// "lang=en-US" for Chrome does not work.

	// Save a few seconds for cleanup
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	bt := s.Param().(testParam).browserType
	br, closeBrowser, err := browserfixt.SetUp(ctx, s.FixtValue(), bt)
	if err != nil {
		s.Fatalf("Failed to open the %v browser: %v", bt, err)
	}
	defer closeBrowser(cleanupCtx)

	// TODO:
	// if err != nil {
	// 	// In case of authentication error, provide a more informative message to the user.
	// 	if strings.Contains(err.Error(), "chrome.Auth") {
	// 		err = errors.Wrap(err, "please supply a password with -var=pass=<password>")
	// 	} else if strings.Contains(err.Error(), "chrome.Contact") {
	// 		err = errors.Wrap(err, "please supply a contact email with -var=contact=<contact>")
	// 	}
	// 	s.Fatal("Failed to start Chrome: ", err)
	// }
	// defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	if err := crd.Launch(ctx, br, tconn); err != nil {
		s.Fatal("Failed to Launch: ", err)
	}

	if shouldWait(s, s.Param().(testParam).manual) {
		s.Log("Waiting connection")
		if err := crd.WaitConnection(ctx, tconn); err != nil {
			s.Fatal("No client connected: ", err)
		}
	} else {
		s.Log("Skip waiting remote connection")
	}
}
