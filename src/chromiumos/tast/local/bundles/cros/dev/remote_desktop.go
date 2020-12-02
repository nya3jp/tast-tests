// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dev

import (
	"context"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

const crdURL = "https://remotedesktop.google.com/support"

// Do not poll things too fast to prevent triggering weird UI behaviors. The
// share button will not work if we keep clicking it.
var rdpPollOpts = &testing.PollOptions{Interval: time.Second}

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
		Desc:         "Connect to Chrome Remote Desktop for working remotely",
		Contacts:     []string{"shik@chromium.org", "tast-users@chromium.org"},
		SoftwareDeps: []string{"chrome"},
		Vars: []string{
			// For running manually.
			"user", "pass", "contact", "wait", "extra_args", "reset",
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

// ensureAppInstalled ensures the companion extension for the Chrome Remote
// Desktop website https://remotedesktop.google.com is installed.
func ensureAppInstalled(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn) error {
	const appCWSURL = "https://chrome.google.com/webstore/detail/chrome-remote-desktop/inomeogfingihgjfjlpeplalcfajhgai?hl=en"

	cws, err := cr.NewConn(ctx, appCWSURL)
	if err != nil {
		return err
	}
	defer cws.Close()
	defer cws.CloseTarget(ctx)

	// Click the add button at most once to prevent triggering
	// weird UI behaviors in Chrome Web Store.
	addClicked := false
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// Check if Remote Desktop is installed.
		params := ui.FindParams{
			Name: "Remove from Chrome",
			Role: ui.RoleTypeButton,
		}
		if installed, err := ui.Exists(ctx, tconn, params); err != nil {
			return testing.PollBreak(err)
		} else if installed {
			return nil
		}

		if !addClicked {
			// If Remote Desktop is not installed, install it now.
			// Click on the add button, if it exists.
			params = ui.FindParams{
				Name: "Add to Chrome",
				Role: ui.RoleTypeButton,
			}
			if addButtonExists, err := ui.Exists(ctx, tconn, params); err != nil {
				return testing.PollBreak(err)
			} else if addButtonExists {
				addButton, err := ui.Find(ctx, tconn, params)
				if err != nil {
					return testing.PollBreak(err)
				}
				defer addButton.Release(ctx)

				if err := addButton.LeftClick(ctx); err != nil {
					return testing.PollBreak(err)
				}
				addClicked = true
			}
		}

		// Click on the confirm button, if it exists.
		params = ui.FindParams{
			Name: "Add extension",
			Role: ui.RoleTypeButton,
		}
		if confirmButtonExists, err := ui.Exists(ctx, tconn, params); err != nil {
			return testing.PollBreak(err)
		} else if confirmButtonExists {
			confirmButton, err := ui.Find(ctx, tconn, params)
			if err != nil {
				return testing.PollBreak(err)
			}
			defer confirmButton.Release(ctx)

			if err := confirmButton.LeftClick(ctx); err != nil {
				return testing.PollBreak(err)
			}
		}
		return errors.New("Remote Desktop still installing")
	}, rdpPollOpts); err != nil {
		return errors.Wrap(err, "failed to install Remote Desktop")
	}
	return nil
}

func launch(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn) (*chrome.Conn, error) {
	// Use english version to avoid i18n differences of HTML element attributes.
	conn, err := cr.NewConn(ctx, crdURL+"?hl=en")
	if err != nil {
		return nil, err
	}

	const waitIdle = "new Promise(resolve => window.requestIdleCallback(resolve))"
	if err := conn.Eval(ctx, waitIdle, nil); err != nil {
		return nil, err
	}

	return conn, nil
}

func getAccessCode(ctx context.Context, crd *chrome.Conn) (string, error) {
	const genCodeBtn = `document.querySelector('[aria-label="Generate Code"]')`
	if err := crd.WaitForExpr(ctx, genCodeBtn); err != nil {
		return "", err
	}

	const clickBtn = genCodeBtn + ".click()"
	if err := crd.Eval(ctx, clickBtn, nil); err != nil {
		return "", err
	}

	const codeSpan = `document.querySelector('[aria-label^="Your access code is:"]')`
	if err := crd.WaitForExpr(ctx, codeSpan); err != nil {
		return "", err
	}

	var code string
	const getCode = codeSpan + `.getAttribute('aria-label').match(/\d+/g).join('')`
	if err := crd.Eval(ctx, getCode, &code); err != nil {
		return "", err
	}
	return code, nil
}

func waitConnection(ctx context.Context, tconn *chrome.TestConn) error {
	// The share button might not be clickable at first, so we keep retrying
	// until we see "Stop Sharing".
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// Check if sharing
		params := ui.FindParams{
			Name: "Stop Sharing",
			Role: ui.RoleTypeButton,
		}
		if sharing, err := ui.Exists(ctx, tconn, params); err != nil {
			return testing.PollBreak(err)
		} else if sharing {
			return nil
		}

		// Click on the share button, if it exists.
		params = ui.FindParams{
			Name: "Share",
			Role: ui.RoleTypeButton,
		}
		if shareButtonExists, err := ui.Exists(ctx, tconn, params); err != nil {
			return testing.PollBreak(err)
		} else if shareButtonExists {
			shareButton, err := ui.Find(ctx, tconn, params)
			if err != nil {
				return testing.PollBreak(err)
			}
			defer shareButton.Release(ctx)

			if err := shareButton.LeftClick(ctx); err != nil {
				return testing.PollBreak(err)
			}
		}
		return errors.New("still enabling sharing")
	}, rdpPollOpts); err != nil {
		return errors.Wrap(err, "failed to enable sharing")
	}
	return nil
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
	opts = append(opts, chrome.Auth(vars.user, vars.pass, ""))
	opts = append(opts, chrome.Contact(vars.contact))
	opts = append(opts, chrome.GAIALogin())

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

	if err := ensureAppInstalled(ctx, cr, tconn); err != nil {
		s.Fatal("Failed to install CRD app: ", err)
	}

	crd, err := launch(ctx, cr, tconn)
	if err != nil {
		s.Fatal("Failed to launch CRD: ", err)
	}
	defer crd.Close()

	s.Log("Getting access code")
	accessCode, err := getAccessCode(ctx, crd)
	if err != nil {
		s.Fatal("Failed to getAccessCode: ", err)
	}
	s.Log("Access code: ", accessCode)
	s.Log("Please paste the code to \"Give Support\" section on ", crdURL)

	if vars.wait {
		s.Log("Waiting connection")
		if err := waitConnection(ctx, tconn); err != nil {
			s.Fatal("No client connected: ", err)
		}
	} else {
		s.Log("Skip waiting remote connection")
	}
}
