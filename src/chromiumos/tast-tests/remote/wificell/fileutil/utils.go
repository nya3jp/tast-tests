// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package fileutil provides utilities for operating files in remote wifi tests.
package fileutil

import (
	"context"
	"os"
	"path"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/ssh"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
)

// WriteTmp writes the content to a temp file created by "mktemp $pattern" on host.
func WriteTmp(ctx context.Context, host *ssh.Conn, pattern string, content []byte) (string, error) {
	out, err := host.CommandContext(ctx, "mktemp", pattern).Output()
	if err != nil {
		return "", errors.Wrap(err, "failed to create temp file on host")
	}
	filepath := strings.TrimSpace(string(out))
	if err := linuxssh.WriteFile(ctx, host, filepath, content, 0644); err != nil {
		return "", err
	}
	return filepath, nil
}

// PrepareOutDirFile prepares the base directory of the path under OutDir and opens the file.
func PrepareOutDirFile(ctx context.Context, filename string) (*os.File, error) {
	outdir, ok := testing.ContextOutDir(ctx)
	if !ok {
		return nil, errors.New("failed to get OutDir")
	}
	filepath := path.Join(outdir, filename)
	if err := os.MkdirAll(path.Dir(filepath), 0755); err != nil {
		return nil, errors.Wrapf(err, "failed to create basedir for %q", filepath)
	}
	f, err := os.OpenFile(filepath, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot open file %q", filepath)
	}
	return f, nil
}
