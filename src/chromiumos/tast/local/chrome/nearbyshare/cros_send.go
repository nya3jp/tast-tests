// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package nearbyshare is used to control Nearby Share functionality.
package nearbyshare

import (
	"context"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// SendSurface is used to control the Nearby Share sending flow.
// The js object implements several Mojo APIs that allow tests to control Nearby Share very closely to how the UI does.
type SendSurface struct {
	conn *chrome.Conn
	js   *chrome.JSObject
}

// Close releases the resources associated with the SendSurface.
func (s *SendSurface) Close(ctx context.Context) error {
	if err := s.js.Call(ctx, nil, `function() {this.stopDiscovery()}`); err != nil {
		return errors.Wrap(err, "failed to stop discovery")
	}
	if err := s.js.Release(ctx); err != nil {
		return errors.Wrap(err, "failed to release reference to JavaScript")
	}
	if err := s.conn.CloseTarget(ctx); err != nil {
		return errors.Wrap(err, "failed to close chrome://nearby/ Chrome target")
	}
	if err := s.conn.Close(); err != nil {
		return errors.Wrap(err, "failed to close chrome://nearby/ conn")
	}
	return nil
}

// We can directly start a share by going to chrome://nearby/share instead of through the share sheet.
// chrome://nearby/share accepts parameters for text shares and file shares.
// Example: share text  - chrome://nearby/share?text=hello
// Example: share file  - chrome://nearby/share?file=/path/to/file
// Example: share files - chrome://nearby/share?file=/path/to/file/1|/path/to/file/2
const (
	chromeNearbyURL = "chrome://nearby/share?"
	textQuery       = "text="
	filesQuery      = "file="
)

// nearbySendJS executes JS on the Chrome session to create an object that implements the necessary
// Mojo interfaces for driving Nearby Share. js should contain callable Javascript that returns the
// desired object.
// The JS used by tests is kept in the data file at src/chromiumos/tast/local/bundles/cros/nearbyshare/data/sender.js
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
	return &sender, nil
}

// waitForProp waits until a property of the SendSurface's JS object is not null.
func (s *SendSurface) waitForProp(ctx context.Context, prop string, timeout time.Duration) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var propExists bool
		if err := s.js.Call(ctx, &propExists, `function(prop) {return this[prop] != null}`, prop); err != nil {
			return testing.PollBreak(err)
		}
		if !propExists {
			return errors.New("property still null")
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout}); err != nil {
		return errors.Wrapf(err, "failed waiting for property %v to not be null", prop)
	}
	return nil
}

// StartSendFiles navigates directly to chrome://nearby to start sharing.
func StartSendFiles(ctx context.Context, cr *chrome.Chrome, senderJSPath string, filepaths []string) (*SendSurface, error) {
	if len(filepaths) < 1 {
		return nil, errors.New("at least one file is required to start sending")
	}
	for _, f := range filepaths {
		if _, err := os.Stat(f); err != nil {
			return nil, errors.Wrapf(err, "file %v does not exist", f)
		}
	}
	url := chromeNearbyURL + filesQuery + strings.Join(filepaths[:], "|")
	sendConn, err := cr.NewConn(ctx, url)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to start Chrome session with url %v", url)
	}

	// Parse the sender JS data file and execute it on the Chrome conn.
	js, err := ioutil.ReadFile(senderJSPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load JS for sending")
	}
	sender, err := nearbySendJS(ctx, sendConn, string(js))
	if err != nil {
		return nil, err
	}

	return &SendSurface{conn: sendConn, js: sender}, nil
}

// WaitForShareTarget waits for the share target with the given name to become available.
func (s *SendSurface) WaitForShareTarget(ctx context.Context, receiverName string, timeout time.Duration) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var targetExists bool
		if err := s.js.Call(ctx, &targetExists, `function(name) {return this.getShareTarget(name) != null}`, receiverName); err != nil {
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
func (s *SendSurface) SelectShareTarget(ctx context.Context, receiverName string) error {
	if err := s.js.Call(ctx, nil, `function(name) {this.selectShareTarget(name)}`, receiverName); err != nil {
		return errors.Wrap(err, "calling selectShareTarget js failed")
	}
	if err := s.waitForProp(ctx, "selectShareTargetRes", 10*time.Second); err != nil {
		return errors.Wrap(err, "selectShareTargetRes still null")
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
func (s *SendSurface) ConfirmationToken(ctx context.Context) (string, error) {
	const tokenProp = "confirmationToken"
	var token string
	if err := s.waitForProp(ctx, tokenProp, 10*time.Second); err != nil {
		return token, err
	}
	if err := s.js.Call(ctx, &token, `function(prop) {return this[prop]}`, tokenProp); err != nil {
		return token, errors.Wrap(err, "failed to get confirmation token")
	}
	return token, nil
}

// CurrentTransferStatus gets the current transfer status.
func (s *SendSurface) CurrentTransferStatus(ctx context.Context) (TransferStatus, error) {
	const statusProp = "currentTransferStatus"
	var status TransferStatus
	if err := s.waitForProp(ctx, statusProp, 10*time.Second); err != nil {
		return status, err
	}
	if err := s.js.Call(ctx, &status, `function(prop) {return this[prop]}`, statusProp); err != nil {
		return status, errors.Wrap(err, "failed to get current transfer status")
	}
	return status, nil
}

// WaitForTransferStatus waits for the specified transfer status.
func (s *SendSurface) WaitForTransferStatus(ctx context.Context, target TransferStatus, timeout time.Duration) error {
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
