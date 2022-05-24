// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cmd

import (
	"context"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/ssh"
)

// FindCmdPath returns full path for the binary on the given device, defined
// by its SSH connection.
func FindCmdPath(ctx context.Context, conn *ssh.Conn, cmd string) (string, error) {
	res, err := conn.CommandContext(ctx, "which", cmd).Output()
	if err != nil {
		return "", errors.Wrapf(err, "failed to run command 'which %s'", cmd)
	}

	return strings.TrimSpace(string(res)), nil
}
