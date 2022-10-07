// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package oobe contains helpers for shared logic in OOBE related tests.
package oobe

import (
	"context"

	"chromiumos/tast/local/chrome"
)

// IsWelcomeScreenVisible checks the current page in OOBE to see if it's
// currently on the welcome page.
func IsWelcomeScreenVisible(ctx context.Context, oobeConn *chrome.Conn) error {
	return oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.WelcomeScreen.isVisible()")
}
