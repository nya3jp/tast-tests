// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package gmail

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// Web provides controls of Gmail website.
type Web struct{}

const gmailURL = "https://mail.google.com"

// NewWeb creates a Web instance and opens Gmail website at start.
func NewWeb(ctx context.Context, cr *chrome.Chrome) (*Web, error) {
	gmail := &Web{}
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create test API connection")
	}
	defer tconn.Close()
	conn, err := cr.NewConn(ctx, gmailURL, cdputil.WithNewWindow())
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open %s", gmailURL)
	}
	if err := webutil.WaitForQuiescence(ctx, conn, time.Minute*2); err != nil {
		return nil, errors.Wrap(err, "failed ailed to wait for page to finish loading")
	}
	cuj.WaitAndClick(ctx, tconn, ui.FindParams{Name: "Got it", Role: ui.RoleTypeButton}, time.Second)

	return gmail, nil
}

// Send sends an email through Gmail website to the specified receiver.
func (*Web) Send(ctx context.Context, cr *chrome.Chrome, receiver, subject, content string) error {
	const timeout = time.Second * 10
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create test API connection")
	}
	defer tconn.Close()

	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, `failed to open the keyboard`)
	}
	defer kb.Close()

	if err := cuj.WaitAndClick(ctx, tconn, ui.FindParams{Name: "Compose"}, timeout); err != nil {
		return errors.Wrap(err, `failed to click 'Compose' button`)
	}

	if err := cuj.WaitAndClick(ctx, tconn, ui.FindParams{Name: "To", Role: ui.RoleTypeTextFieldWithComboBox}, timeout); err != nil {
		return errors.Wrap(err, `failed to click 'To' field`)
	}

	if err := kb.Type(ctx, receiver); err != nil {
		return errors.Wrap(err, `failed to do keyboard type`)
	}

	if err := kb.Accel(ctx, "Enter"); err != nil {
		return errors.Wrap(err, `failed to send keyboard event`)
	}

	if err := cuj.WaitAndClick(ctx, tconn, ui.FindParams{Name: "Subject"}, timeout); err != nil {
		return errors.Wrap(err, `failed to click 'Subject' field`)
	}
	if err := kb.Type(ctx, subject); err != nil {
		return errors.Wrap(err, `failed to do keyboard type`)
	}

	if err := cuj.WaitAndClick(ctx, tconn, ui.FindParams{Name: "Message Body"}, timeout); err != nil {
		return errors.Wrap(err, `failed to click 'Message Body' field`)
	}

	if err := kb.Type(ctx, content); err != nil {
		return errors.Wrap(err, `failed to do keyboard type`)
	}

	//Send (Ctrl-Enter)
	if err := kb.Accel(ctx, "Ctrl+Enter"); err != nil {
		return errors.Wrap(err, `failed to send keyboard event`)
	}

	// Wait for sending mail.
	if err := testing.Sleep(ctx, time.Second); err != nil {
		return errors.Wrap(err, "failed to wait")
	}

	return nil
}
