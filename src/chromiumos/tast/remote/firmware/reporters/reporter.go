// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package reporters

import (
	"context"
	"fmt"
	"strings"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
)

type reporter struct {
	d *dut.DUT
}

// New creates a reporter.
func New(d *dut.DUT) *reporter {
	return &reporter{d}
}

// Parse file contents by line and report the list of lines.
func (r *reporter) CatFileLines(ctx context.Context, path string) ([]string, error) {
	res, err := r.CatFile(ctx, path)
	if err != nil {
		return nil, err
	}
	return strings.Split(string(res), "\n"), nil
}

// Parse command output by line and report the list of lines.
func (r *reporter) CommandLines(ctx context.Context, format string, args ...string) ([]string, error) {
	res, err := r.Command(ctx, format, args...)
	if err != nil {
		return nil, err
	}
	return strings.Split(string(res), "\n"), nil
}

// Report the file contents as a single string.
func (r *reporter) CatFile(ctx context.Context, path string) (string, error) {
	res, err := r.Command(ctx, "cat", path)
	if err != nil {
		return "", err
	}
	return res, nil
}

// Report the command output as a single string.
func (r *reporter) Command(ctx context.Context, format string, args ...string) (string, error) {
	res, err := r.d.Conn().Command(format, args...).Output(ctx)
	if err != nil {
		return "", errors.Wrapf(err, "failed to run %q command on dut", fmt.Sprintf(format, args))
	}

	// Command returns an extra newline vs running the command in shell, so remove it.
	if len(res) > 0 && res[len(res)-1] == '\n' {
		return string(res[:len(res)-1]), nil
	}
	return string(res), nil
}
