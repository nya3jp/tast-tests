// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package reporters

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"chromiumos/tast/errors"
)

// CommandOutputLines parses command output by line and report the list of lines.
func (r *Reporter) CommandOutputLines(ctx context.Context, format string, args ...string) ([]string, error) {
	res, err := r.CommandOutput(ctx, format, args...)
	if err != nil {
		return nil, err
	}
	return strings.Split(res, "\n"), nil
}

// CommandOutput reports the command output as a single string.
func (r *Reporter) CommandOutput(ctx context.Context, format string, args ...string) (string, error) {
	res, err := r.d.Conn().Command(format, args...).Output(ctx)
	if err != nil {
		return "", errors.Wrapf(err, "failed to run %q command on dut", fmt.Sprintf(format, args))
	}

	// Command returns an extra newline vs running the command in shell, so remove it.
	return string(bytes.TrimSuffix(res, []byte{'\n'})), nil
}
