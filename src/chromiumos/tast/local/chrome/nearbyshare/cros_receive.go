// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package nearbyshare is used to control Nearby Share functionality.
package nearbyshare

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
)

// ReceiveSurface is used to control the Nearby Share high-visibility receiving flow.
// The js object implements several Mojo APIs that allow tests to control Nearby Share very closely to how the UI does.
type ReceiveSurface struct {
	conn *chrome.Conn
}

// Close releases the resources associated with the ReceiveSurface.
func (r *ReceiveSurface) Close(ctx context.Context) error {
	if err := r.conn.CloseTarget(ctx); err != nil {
		return errors.Wrap(err, "failed to close chrome://nearby/ Chrome target")
	}
	if err := r.conn.Close(); err != nil {
		return errors.Wrap(err, "failed to close chrome://os-settings/ conn")
	}
	return nil
}

// High-visibility receiving is implemented in the Nearby Share page of OS settings.
// We can control receiving with the page's settings-nearby-share-subpage and nearby-share-receive-dialog elements.
// TODO(crbug/1173190): Replace with public test functions when available.
const (
	showHighVisJS        = nearbySettingsSubpageJS + `.showHighVisibilityPage_()`
	receiveDialogJS      = nearbySettingsSubpageJS + `.shadowRoot.querySelector("nearby-share-receive-dialog")`
	receiveShareTargetJS = receiveDialogJS + `.shareTarget`
	receiveTokenJS       = receiveDialogJS + `.connectionToken`
	receiveAcceptJS      = receiveDialogJS + `.onAccept_()`
)

// StartReceiving initiates high-visibility receiving from chrome://os-settings.
func StartReceiving(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome) (*ReceiveSurface, error) {
	settingsConn, err := nearbySettingsConn(ctx, tconn, cr)
	if err != nil {
		return nil, err
	}
	receiveSurface := &ReceiveSurface{conn: settingsConn}

	// Start high-vis receiving.
	if err := receiveSurface.conn.Eval(ctx, showHighVisJS, nil); err != nil {
		return nil, errors.Wrap(err, "failed to set nearby subpage's inHighVisibility_ property")
	}

	return receiveSurface, nil
}

// WaitForSender waits until the specified sender is detected, and returns the confirmation token.
func (r *ReceiveSurface) WaitForSender(ctx context.Context, senderName string, timeout time.Duration) (string, error) {
	if err := r.conn.WaitForExprFailOnErrWithTimeout(ctx, receiveShareTargetJS, timeout); err != nil {
		return "", errors.Wrap(err, "timed out waiting to detect sender")
	}

	var name string
	if err := r.conn.Eval(ctx, receiveShareTargetJS+`.name`, &name); err != nil {
		return "", errors.Wrap(err, "failed to get share target name")
	}
	if name != senderName {
		return "", errors.Errorf("discovered share target's name does not match the sender; expected %v, got %v", senderName, name)
	}

	var token string
	if err := r.conn.Eval(ctx, receiveTokenJS, &token); err != nil {
		return "", errors.Wrap(err, "failed to get confirmation token")
	}
	return token, nil
}

// AcceptShare accepts the incoming share.
func (r *ReceiveSurface) AcceptShare(ctx context.Context) error {
	return r.conn.Eval(ctx, receiveAcceptJS, nil)
}
