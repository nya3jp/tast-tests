// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"
	"strconv"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/testing"
)

// rdpVars represents the configurable parameters for the remote desktop session.
type rdpVars struct {
	user string
	pass string
	wait bool
}

func init() {
	// Example usage:
	// $ tast run -var=user=<username> -var=pass=<password> <dut ip> dev.RemoteDesktop
	// <username> and <password> are the credentials of the test GAIA account.
	testing.AddTest(&testing.Test{
		Func:         RemoteDesktop,
		Desc:         "Connect to Chrome Remote Desktop for working remotely",
		Contacts:     []string{"shik@chromium.org", "tast-users@chromium.org"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome"},
		Vars: []string{
			// For running manually.
			"user", "pass", "wait",
			// For automated testing.
			"dev.username", "dev.password",
		},
	})
}

// ensureAppInstalled ensures the companion extension for the Chrome Remote
// Desktop website https://remotedesktop.google.com is installed.
func ensureAppInstalled(ctx context.Context, cr *chrome.Chrome, tconn *chrome.Conn) error {
	const appCWSURL = "https://chrome.google.com/webstore/detail/chrome-remote-desktop/inomeogfingihgjfjlpeplalcfajhgai?hl=en"

	cws, err := cr.NewConn(ctx, appCWSURL)
	if err != nil {
		return err
	}
	defer cws.Close()
	defer cws.CloseTarget(ctx)

	// Get UI root.
	root, err := ui.Root(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get UI automation root")
	}
	defer root.Release(ctx)

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// Check if Remote Desktop is installed.
		params := ui.FindParams{
			Name: "Remove from Chrome",
			Role: ui.RoleTypeButton,
		}
		if installed, err := root.DescendantExists(ctx, params); err != nil {
			return testing.PollBreak(err)
		} else if installed {
			return nil
		}

		// If Remote Desktop is not installed, install it now.
		// Click on the add button, if it exists.
		params = ui.FindParams{
			Name: "Add to Chrome",
			Role: ui.RoleTypeButton,
		}
		if addButtonExists, err := root.DescendantExists(ctx, params); err != nil {
			return testing.PollBreak(err)
		} else if addButtonExists {
			addButton, err := root.Descendant(ctx, params)
			if err != nil {
				return testing.PollBreak(err)
			}
			defer addButton.Release(ctx)

			if err := addButton.LeftClick(ctx); err != nil {
				return testing.PollBreak(err)
			}
		}

		// Click on the confirm button, if it exists.
		params = ui.FindParams{
			Name: "Add extension",
			Role: ui.RoleTypeButton,
		}
		if confirmButtonExists, err := root.DescendantExists(ctx, params); err != nil {
			return testing.PollBreak(err)
		} else if confirmButtonExists {
			confirmButton, err := root.Descendant(ctx, params)
			if err != nil {
				return testing.PollBreak(err)
			}
			defer confirmButton.Release(ctx)

			if err := confirmButton.LeftClick(ctx); err != nil {
				return testing.PollBreak(err)
			}
		}
		return errors.New("Remote Desktop still installing")
	}, nil); err != nil {
		return errors.Wrap(err, "failed to install Remote Desktop")
	}
	return nil
}

func launch(ctx context.Context, cr *chrome.Chrome, tconn *chrome.Conn) (*chrome.Conn, error) {
	conn, err := cr.NewConn(ctx, "https://remotedesktop.google.com/support?hl=en")
	if err != nil {
		return nil, err
	}

	const waitIdle = "new Promise(resolve => window.requestIdleCallback(resolve))"
	if err := conn.EvalPromise(ctx, waitIdle, nil); err != nil {
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
	if err := crd.Exec(ctx, clickBtn); err != nil {
		return "", err
	}

	const codeSpan = `document.querySelector('[aria-label^="Your access code is:"]')`
	if err := crd.WaitForExpr(ctx, codeSpan); err != nil {
		return "", err
	}

	var code string
	const getCode = codeSpan + `.getAttribute('aria-label').match(/\d+/g).join('')`
	if err := crd.EvalPromise(ctx, getCode, &code); err != nil {
		return "", err
	}
	return code, nil
}

func waitConnection(ctx context.Context, tconn *chrome.Conn) error {
	// Get UI root.
	root, err := ui.Root(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get UI automation root")
	}
	defer root.Release(ctx)

	// The share button might not be clickable at first, so we keep retrying
	// until we see "Stop Sharing".
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// Check if sharing
		params := ui.FindParams{
			Name: "Stop Sharing",
			Role: ui.RoleTypeButton,
		}
		if sharing, err := root.DescendantExists(ctx, params); err != nil {
			return testing.PollBreak(err)
		} else if sharing {
			return nil
		}

		// Click on the share button, if it exists.
		params = ui.FindParams{
			Name: "Share",
			Role: ui.RoleTypeButton,
		}
		if shareButtonExists, err := root.DescendantExists(ctx, params); err != nil {
			return testing.PollBreak(err)
		} else if shareButtonExists {
			shareButton, err := root.Descendant(ctx, params)
			if err != nil {
				return testing.PollBreak(err)
			}
			defer shareButton.Release(ctx)

			if err := shareButton.LeftClick(ctx); err != nil {
				return testing.PollBreak(err)
			}
		}
		return errors.New("still enabling sharing")
	}, nil); err != nil {
		return errors.Wrap(err, "failed to enable sharing")
	}
	return nil
}

// getVars extracts the testing parameters from testing.State. The user
// provided credentials would override the credentials from config file.
func getVars(s *testing.State) rdpVars {
	manual := true

	user, ok := s.Var("user")
	if !ok {
		manual = false
		user = s.RequiredVar("dev.username")
	}

	pass, ok := s.Var("pass")
	if !ok {
		manual = false
		pass = s.RequiredVar("dev.password")
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

	return rdpVars{
		user: user,
		pass: pass,
		wait: wait,
	}
}

func RemoteDesktop(ctx context.Context, s *testing.State) {
	// TODO(shik): The button names only work in English locale, and adding
	// "lang=en-US" for Chrome does not work.

	vars := getVars(s)

	chromeARCOpt := chrome.ARCDisabled()
	if arc.Supported() {
		chromeARCOpt = chrome.ARCSupported()
	}
	cr, err := chrome.New(
		ctx,
		chromeARCOpt,
		chrome.Auth(vars.user, vars.pass, ""),
		chrome.GAIALogin(),
		chrome.KeepState(),
	)
	if err != nil {
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

	if vars.wait {
		s.Log("Waiting connection")
		if err := waitConnection(ctx, tconn); err != nil {
			s.Fatal("No client connected: ", err)
		}
	} else {
		s.Log("Skip waiting remote connection")
	}
}
