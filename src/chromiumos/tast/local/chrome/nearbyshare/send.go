// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package nearbyshare is used to control Nearby Share functionality.
package nearbyshare

import (
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
	"context"
	"strings"
	"time"
)

// SendSurface is used to control the Nearby Share sending flow.
// The js object implements several Mojo APIs that allow tests to control Nearby Share very closely to how the UI does.
type SendSurface struct {
    conn *chrome.Conn
    js   *chrome.JSObject
}

func (s *SendSurface) Close() error {
	return s.conn.Close()
}

const (
    chromeNearbyURL = "chrome://nearby/share?"
    textQuery       = "text="
    filesQuery      = "file="
)

func nearbySendJS(ctx context.Context, conn *chrome.Conn, js string) (*chrome.JSObject, error) {
	// Wait for Nearby Share Mojo APIs to become available.
	if err := conn.WaitForExpr(ctx, `nearbyShare.mojom !== undefined`); err != nil {
		return nil, errors.Wrap(err, "failed waiting for nearbyShare.mojom to load")
	}

	// Set up a JS object on the Chrome session for controlling sending.
	var sender chrome.JSObject
	if err := conn.Call(ctx, &sender, js); err != nil {
		return nil, errors.Wrap(err, "failed to set up the sender test object")
	}

	// Start discovery.
	if err := sender.Call(ctx, nil, `async function() {await this.startDiscovery()}`); err != nil {
		return nil, errors.Wrap(err, "failed to start discovery")
	}

	// // Check the result to ensure discovery was started successfully.
	// var res startDiscoveryResult
	// if err := sender.Call(ctx, &res, `function() {return this.startDiscoveryRes}`); err != nil {
	// 	return nil, errors.Wrap(err, "failed to get startDiscovery result")
	// }

	// switch res {
	// case startDiscoveryResultErrorInProgressTransferring:
	// 	return nil, errors.New("existing file transfer in progress (kErrorInProgressTransferring)")
	// case startDiscoveryResultErrorGeneric:
	// 	return nil, errors.New("unable to start discovery (kErrorGeneric)")
	// }
	return &sender, nil
}

// StartSendFiles navigates directly to chrome://nearby to start sharing.
func StartSendFiles(ctx context.Context, cr *chrome.Chrome, senderJS string, filepaths []string) (*SendSurface, error) {
	url := chromeNearbyURL+filesQuery+strings.Join(filepaths[:], "|")
	testing.ContextLog(ctx, "========= nearby URL: ", url)
	sendConn, err := cr.NewConn(ctx, url)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to start Chrome session with url %v", url)
	}

	sender, err := nearbySendJS(ctx, sendConn, senderJS)
	if err != nil {
		return nil, err
	}

	return &SendSurface{conn: sendConn, js: sender}, nil
}

// WaitForShareTarget waits for the share target with the given name to become available.
func (s* SendSurface) WaitForShareTarget(ctx context.Context, receiverName string, timeout time.Duration) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var targetExists bool
		if err := s.js.Call(ctx, &targetExists, `function(name) {return this.shareTargetNameMap.get(name) != undefined}`, receiverName); err != nil {
			return testing.PollBreak(err)
		}
		if !targetExists {
			return errors.New("share target not found yet")
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout}); err != nil {
		return errors.Wrap(err, "failed waiting to find the share target")
	}
	return nil
}

// SelectShareTarget selects the specified device as a receiver and initiates the share. The transfer will begin pending the receiver's confirmation.
func (s* SendSurface) SelectShareTarget(ctx context.Context, receiverName string) error {
	if err := s.js.Call(ctx, nil, `async function(name) {await this.selectShareTarget(name)}`, receiverName); err != nil {
		return errors.Wrap(err, "calling selectShareTarget js failed")
	}
	// Confirm selecting the share target was successful.
	var res selectShareTargetResult
	if err := s.js.Call(ctx, &res, `function() {return this.selectShareTargetRes}`); err != nil {
		return errors.Wrap(err, "failed to get selectShareTargetRes")
	}

	switch res {
	case selectShareTargetResultInvalidShareTarget:
		return errors.New("could not find the selected share target (kInvalidShareTarget)")
	case selectShareTargetResultError:
		return errors.New("unknown error when selecting share target (kError)")
	}
	return nil
}

// ConfirmationToken gets the secure sharing token for the transfer.
func (s* SendSurface) ConfirmationToken(ctx context.Context) (string, error) {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var tokenExists bool
		if err := s.js.Call(ctx, &tokenExists, `function(name) {return this.confirmationToken != null}`); err != nil {
			return testing.PollBreak(err)
		}
		if !tokenExists {
			return errors.New("confirmation token not found yet")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10*time.Second}); err != nil {
		return "", errors.Wrap(err, "failed waiting for the confirmation token to be registered")
	}
	var token string
	if err := s.js.Call(ctx, &token, `function() {return this.confirmationToken}`); err != nil {
		return "", errors.Wrap(err, "failed to get confirmation token")
	}
	return token, nil
}

// CurrentTransferStatus gets the current transfer status.
func (s* SendSurface) CurrentTransferStatus(ctx context.Context) (TransferStatus, error) {
	var status TransferStatus
	err := s.js.Call(ctx, &status, `function() {return this.currentTransferStatus}`)
	return status, err
}

// WaitForTransferStatus waits for the specified transfer status.
func (s* SendSurface) WaitForTransferStatus(ctx context.Context, target TransferStatus, timeout time.Duration) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		current, err := s.CurrentTransferStatus(ctx)
		if err != nil {
			testing.PollBreak(err)
		}
		if current != target {
			return errors.Errorf("target status not reached yet; current status %v", current)
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout}); err != nil {
		return errors.Wrapf(err, "failed waiting for target status %v", target)
	}
	return nil
}