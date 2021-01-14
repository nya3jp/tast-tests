// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package crd provides utilities to set up Chrome Remote Desktop.
package crd

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/testing"
)

const crdURL = "https://remotedesktop.google.com/support"

// Do not poll things too fast to prevent triggering weird UI behaviors. The
// share button will not work if we keep clicking it. Sets timeout to 5 minutes
// according to timeout for CRD one time access code.
var rdpPollOpts = &testing.PollOptions{Interval: time.Second, Timeout: 5 * time.Minute}

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

// Launch prepares Chrome Remote Desktop and generates access code to be connected by.
func Launch(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn) error {
	if err := ensureAppInstalled(ctx, cr, tconn); err != nil {
		return errors.Wrap(err, "failed to install CRD app")
	}

	crd, err := launch(ctx, cr, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to launch CRD")
	}
	defer crd.Close()

	testing.ContextLog(ctx, "Getting access code")
	accessCode, err := getAccessCode(ctx, crd)
	if err != nil {
		return errors.Wrap(err, "failed to getAccessCode")
	}
	testing.ContextLog(ctx, "Access code: ", accessCode)
	testing.ContextLog(ctx, "Please paste the code to \"Give Support\" section on ", crdURL)

	return nil
}

// WaitConnection waits for remote desktop client connecting to DUT.
func WaitConnection(ctx context.Context, tconn *chrome.TestConn) error {
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
