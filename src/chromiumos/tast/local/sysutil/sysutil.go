// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package sysutil provides utilities for getting system-related information.
package sysutil

import (
	"os/user"
	"strconv"

	"golang.org/x/sys/unix"

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

// Utsname contains system information.
type Utsname struct {
	// Operating system name (e.g., "Linux").
	// Corresponds to `uname -s`.
	Sysname string
	// Name within "some implementation-defined network".
	// Corresponds to `uname -n`.
	Nodename string
	// Operating system release (e.g., "2.6.28").
	// Corresponds to `uname -r`.
	Release string
	// Operating system version.
	// Corresponds to `uname -v`.
	Version string
	// Hardware identifier (e.g., "x86_64").
	// Corresponds to `uname -m`.
	Machine string
	// NIS or YP domain name. This is nonstandard GNU/Linux extension.
	Domainname string
}

var unameFunc func(*unix.Utsname) error = unix.Uname

// Uname is a wrapper of uname unix. See man 2 uname.
func Uname() (*Utsname, error) {
	u := unix.Utsname{}
	if err := unameFunc(&u); err != nil {
		return nil, err
	}
	var res Utsname

	for _, c := range u.Sysname {
		if c == 0 {
			break
		}
		res.Sysname += string(int(c))
	}
	for _, c := range u.Nodename {
		if c == 0 {
			break
		}
		res.Nodename += string(int(c))
	}
	for _, c := range u.Release {
		if c == 0 {
			break
		}
		res.Release += string(int(c))
	}
	for _, c := range u.Version {
		if c == 0 {
			break
		}
		res.Version += string(int(c))
	}
	for _, c := range u.Machine {
		if c == 0 {
			break
		}
		res.Machine += string(int(c))
	}
	for _, c := range u.Domainname {
		if c == 0 {
			break
		}
		res.Domainname += string(int(c))
	}
	return &res, nil
}
