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
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/cws"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

const (
	crdURL    = "https://remotedesktop.google.com/support"
	appCWSURL = "https://chrome.google.com/webstore/detail/chrome-remote-desktop/inomeogfingihgjfjlpeplalcfajhgai?hl=en"
)

// Do not poll things too fast to prevent triggering weird UI behaviors. The
// share button will not work if we keep clicking it. Sets timeout to 5 minutes
// according to timeout for CRD one time access code.
var rdpPollOpts = &testing.PollOptions{Interval: time.Second, Timeout: 5 * time.Minute}

func launch(ctx context.Context, br *browser.Browser, tconn *chrome.TestConn) (*chrome.Conn, error) {
	// Use english version to avoid i18n differences of HTML element attributes.
	conn, err := br.NewConn(ctx, crdURL+"?hl=en")
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
func Launch(ctx context.Context, br *browser.Browser, tconn *chrome.TestConn) error {
	// Ensures the companion extension for the Chrome Remote Desktop website
	// https://remotedesktop.google.com is installed.
	app := cws.App{Name: "Remote Desktop", URL: appCWSURL}
	if err := cws.InstallApp(ctx, br, tconn, app); err != nil {
		return errors.Wrap(err, "failed to install CRD app")
	}

	crd, err := launch(ctx, br, tconn)
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
	ui := uiauto.New(tconn).WithPollOpts(*rdpPollOpts)
	share := nodewith.Name("Share").Role(role.Button)
	shared := ui.Exists(nodewith.Name("Stop Sharing").Role(role.Button))
	if err := ui.LeftClickUntil(share, shared)(ctx); err != nil {
		return errors.Wrap(err, "failed to enable sharing")
	}
	return nil
}
