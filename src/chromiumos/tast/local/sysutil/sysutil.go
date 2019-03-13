// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package sysutil provides utilities for getting system-related information.
package sysutil

import (
	"os/user"
	"strconv"

	"chromiumos/tast/errors"
)

const (
	// ChronosUID is the UID of the user "chronos".
	// A constant is defined since this is unlikely to be changed and since
	// it simplifies tests.
	ChronosUID uint32 = 1000

	// ChronosGID is the GID of the group "chronos", similar to ChronosUID.
	ChronosGID uint32 = 1000
)

// GetUID returns the UID corresponding to username.
func GetUID(username string) (uint32, error) {
	u, err := user.Lookup(username)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to look up user %q", username)
	}
	uid, err := strconv.ParseInt(u.Uid, 10, 64)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to parse UID %q for user %q", u.Uid, username)
	}
	return uint32(uid), nil
}

// GetGID returns the GID corresponding to group.
func GetGID(group string) (uint32, error) {
	g, err := user.LookupGroup(group)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to look up group %q", group)
	}
	gid, err := strconv.ParseInt(g.Gid, 10, 64)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to parse GID %q for group %q", g.Gid, group)
	}
	return uint32(gid), nil
}
