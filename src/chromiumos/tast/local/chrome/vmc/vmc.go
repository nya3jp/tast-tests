// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package vmc provides utilities for the vmc command.
package vmc

import (
	"context"
	"os"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/session"
)

// UserIDHash returns a sanitized username of the primary session.
// The return value can be used as "CROS_USER_ID_HASH".
func UserIDHash(ctx context.Context) (string, error) {
	sessionManager, err := session.NewSessionManager(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to start session manager")
	}

	_, hash, err := sessionManager.RetrievePrimarySession(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to retrieve the primary session")
	}

	return hash, nil
}

// Command creates a vmc testexec command.
func Command(ctx context.Context, hash string, arg ...string) *testexec.Cmd {
	cmd := testexec.CommandContext(ctx, "vmc", arg...)
	cmd.Env = append(
		os.Environ(),
		"CROS_USER_ID_HASH="+hash,
	)
	return cmd
}
