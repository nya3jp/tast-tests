// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package unicorn provides Family Link user login functions.
package unicorn

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
)

// LoginAsRegularOrChild logs in as regular user or child.
// `child` set to true for child login, false for regular user login.
func LoginAsRegularOrChild(ctx context.Context, parentUser, parentPass, childUser, childPass string, child bool, args ...chrome.Option) (*chrome.Chrome, *chrome.TestConn, error) {
	args = append(args, chrome.GAIALogin())
	if child {
		args = append(args, chrome.Auth(childUser, childPass, "gaia-id"), chrome.ParentAuth(parentUser, parentPass))
	} else {
		args = append(args, chrome.Auth(parentUser, parentPass, "gaia-id"))
	}
	cr, err := chrome.New(ctx, args...)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to start Chrome: ")
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to created test API connection: ")
	}

	return cr, tconn, nil
}
