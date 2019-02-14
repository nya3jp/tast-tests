// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package fsinfo reports information about filesystems on behalf of tests.
package fsinfo

import (
	"bytes"
	"context"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
)

// Info contains information about a filesystem.
type Info struct {
	// Dev contains the underlying filesystem device, e.g. "/dev/root".
	Dev string
	// Type contains the filesystem type, e.g. "ext2".
	Type string
	// Used contains the number of used bytes.
	Used int64
	// Avail contains the number of available bytes.
	Avail int64
}

// Get returns information about the filesystem mounted at mountPoint, e.g. "/".
func Get(ctx context.Context, mountPoint string) (*Info, error) {
	cmd := testexec.CommandContext(ctx, "df", "-B1", "--print-type", mountPoint)
	out, err := cmd.Output()
	if err != nil {
		cmd.DumpLog(ctx)
		return nil, errors.Wrap(err, "df command failed")
	}
	return parseDfOutput(out)
}

// parseDfOutput parses output from a "df -B1 --print-type <mountpoint>" command.
//
// The output is expected to have the following form:
//  Filesystem     Type  1B-blocks       Used Available Use% Mounted on
//  /dev/root      ext2 2064203776 1718403072 345800704  84% /
func parseDfOutput(out []byte) (*Info, error) {
	out = bytes.TrimSpace(out)
	if len(out) == 0 {
		return nil, errors.New("df produced no output")
	}

	lines := strings.Split(string(out), "\n")
	last := lines[len(lines)-1]
	fields := strings.Fields(last)
	if len(fields) != 7 {
		return nil, errors.Errorf("expected 7 fields in line %q", last)
	}

	info := &Info{
		Dev:  fields[0],
		Type: fields[1],
	}
	var err error
	if info.Used, err = strconv.ParseInt(fields[3], 10, 64); err != nil {
		return nil, errors.Wrapf(err, "failed to parse 'used' value %q", fields[3])
	}
	if info.Avail, err = strconv.ParseInt(fields[4], 10, 64); err != nil {
		return nil, errors.Wrapf(err, "failed to parse 'avail' value %q", fields[4])
	}
	return info, nil
}
