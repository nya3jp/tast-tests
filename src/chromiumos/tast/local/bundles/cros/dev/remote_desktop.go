// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dev

import (
	"context"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/crd"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// rdpVars represents the configurable parameters for the remote desktop session.
type rdpVars struct {
	user      string
	pass      string
	contact   string
	wait      bool
	reset     bool
	extraArgs []string
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
			"user", "pass", "contact", "wait", "extra_args", "reset",
		},
		VarDeps: []string{
			// For automated testing.
			"dev.username", "dev.password",
		},
		Params: []testing.Param{{
			// For running manually.
			Name: "",
		}, {
			// For automated testing.
			Name:      "test",
			ExtraAttr: []string{"group:mainline", "informational"},
			// TODO(b/151111783): This is a speculative fix to limit the number of sessions. It
			// seems that the test account is throttled by the CRD backend, so the test is failing
			// with a periodic pattern. The model list is handcrafted to cover various platforms.
			ExtraHardwareDeps: hwdep.D(hwdep.Model("atlas", "careena", "dru", "eve", "kohaku",
				"krane", "nocturne")),
		}},
	})
}

// getVars extracts the testing parameters from testing.State. The user
// provided credentials would override the credentials from config file.
func getVars(s *testing.State) rdpVars {
	user, hasUser := s.Var("user")
	if !hasUser {
		user = s.RequiredVar("dev.username")
	}

	pass, hasPass := s.Var("pass")
	if !hasPass {
		pass = s.RequiredVar("dev.password")
	}

	contact, hasContact := s.Var("contact")

	// Manual test requires username + password/contact.
	manual := false
	if hasUser && (hasPass || hasContact) {
		manual = true
	}

	resetStr, ok := s.Var("reset")
	if !ok {
		resetStr = "false"
	}
	reset, err := strconv.ParseBool(resetStr)
	if err != nil {
		s.Fatal("Failed to parse the variable `reset`: ", err)
	}

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

	extraArgsStr, ok := s.Var("extra_args")
	if !ok {
		extraArgsStr = ""
	}
	extraArgs := strings.Fields(extraArgsStr)

	return rdpVars{
		user:      user,
		pass:      pass,
		contact:   contact,
		wait:      wait,
		reset:     reset,
		extraArgs: extraArgs,
	}
}

func RemoteDesktop(ctx context.Context, s *testing.State) {
	// TODO(shik): The button names only work in English locale, and adding
	// "lang=en-US" for Chrome does not work.

	vars := getVars(s)
	var opts []chrome.Option

	chromeARCOpt := chrome.ARCDisabled()
	if arc.Supported() {
		chromeARCOpt = chrome.ARCSupported()
	}
	opts = append(opts, chromeARCOpt)
	opts = append(opts, chrome.GAIALogin(chrome.Creds{
		User:    vars.user,
		Pass:    vars.pass,
		Contact: vars.contact,
	}))

	if !vars.reset {
		opts = append(opts, chrome.KeepState())
	}

	if len(vars.extraArgs) > 0 {
		opts = append(opts, chrome.ExtraArgs(vars.extraArgs...))
	}

	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		// In case of authentication error, provide a more informative message to the user.
		if strings.Contains(err.Error(), "chrome.Auth") {
			err = errors.Wrap(err, "please supply a password with -var=pass=<password>")
		} else if strings.Contains(err.Error(), "chrome.Contact") {
			err = errors.Wrap(err, "please supply a contact email with -var=contact=<contact>")
		}
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	if err := crd.Launch(ctx, cr.Browser(), tconn); err != nil {
		s.Fatal("Failed to Launch: ", err)
	}

	if vars.wait {
		s.Log("Waiting connection")
		if err := crd.WaitConnection(ctx, tconn); err != nil {
			s.Fatal("No client connected: ", err)
		}
	} else {
		s.Log("Skip waiting remote connection")
	}
}
