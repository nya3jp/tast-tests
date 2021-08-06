// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package nearbyshare is used to control Chrome OS Nearby Share functionality.
package nearbyshare

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
)

// SendSurface is used to control the Nearby Share sending flow on Chrome OS.
// The js object implements several Mojo APIs that allow tests to control Nearby Share very closely to how the UI does.
type SendSurface struct {
	conn *chrome.Conn
}

// Close releases the resources associated with the SendSurface.
func (s *SendSurface) Close(ctx context.Context) error {
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

// StartSendFiles navigates directly to chrome://nearby to start sharing.
func StartSendFiles(ctx context.Context, cr *chrome.Chrome, filepaths []string) (*SendSurface, error) {
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

	return &SendSurface{conn: sendConn}, nil
}

// JavaScript for interacting with the discovery page. All of the properties and methods defined by the page
// are accessible through the nearby-discovery-page element.
// TODO(crbug/1170815): Replace with public test functions when available.
const discoveryElementJS = `document.querySelector("nearby-share-app").shadowRoot.querySelector("nearby-discovery-page")`
const selectedShareTargetJS = discoveryElementJS + `.selectedShareTarget`
const onNextJS = discoveryElementJS + `.onNext_()`
const onCancelJS = discoveryElementJS + `.shadowRoot.querySelector("nearby-page-template").shadowRoot.querySelector("#cancelButton").click()`
const onCancelJS2 = discoveryElementJS + `.shadowRoot.querySelector("nearby-page-template").shadowRoot.getElementById("cancelButton").click()`
const onCancelJS3 = discoveryElementJS + `.shadowRoot.querySelector("nearby-page-template").onCancelClick_()`

func findShareTargetJS(name string) string {
	return fmt.Sprintf(discoveryElementJS+`.shareTargets_.find(t => t.name == %q)`, name)
}

// WaitForShareTarget waits for the share target with the given name to become available.
func (s *SendSurface) WaitForShareTarget(ctx context.Context, receiverName string, timeout time.Duration) error {
	return s.conn.WaitForExprFailOnErrWithTimeout(ctx, findShareTargetJS(receiverName), timeout)
}

// SelectShareTarget selects the specified device as a receiver and initiates the share.
// The transfer will begin pending the receiver's confirmation.
// The timeout specifies how long to wait for the receiver to be found in the list of available share targets.
func (s *SendSurface) SelectShareTarget(ctx context.Context, receiverName string, timeout time.Duration) error {
	if err := s.WaitForShareTarget(ctx, receiverName, timeout); err != nil {
		return errors.Wrap(err, "failed to wait for share target")
	}
	if err := s.conn.Eval(ctx, selectedShareTargetJS+`=`+findShareTargetJS(receiverName), nil); err != nil {
		return errors.Wrap(err, "failed to assign selectedShareTarget")
	}
	if err := s.conn.Eval(ctx, onNextJS, nil); err != nil {
		return errors.Wrap(err, "failed to call onNext to transition to confirmation")
	}
	return nil
}

// Cancel transfer.
func (s *SendSurface) Cancel(ctx context.Context) error {
	if err := s.conn.Eval(ctx, onCancelJS3, nil); err != nil {
		return errors.Wrap(err, "failed to call onCancel to cancel the transfer")
	}
	return nil
}

// JavaScript for interacting with the confirmation page. All of the properties and methods defined by the page
// are accessible through the nearby-confirmation-page element.
// TODO(crbug/1170815): Replace with public test functions when available.
const confirmationElementJS = `document.querySelector("nearby-share-app").shadowRoot.querySelector("nearby-confirmation-page")`
const confirmationTokenJS = confirmationElementJS + `.confirmationToken_`

// ConfirmationToken gets the secure sharing token for the transfer.
func (s *SendSurface) ConfirmationToken(ctx context.Context) (string, error) {
	if err := s.conn.WaitForExpr(ctx, confirmationTokenJS); err != nil {
		return "", errors.Wrap(err, "failed waiting for valid confirmation token")
	}
	var token string
	if err := s.conn.Eval(ctx, confirmationTokenJS, &token); err != nil {
		return "", errors.Wrap(err, "failed to get confirmation token")
	}
	return token, nil
}
