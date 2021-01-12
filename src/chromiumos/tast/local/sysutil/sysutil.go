// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package sysutil provides utilities for getting system-related information.
package sysutil

import (
	"bytes"
	"os/user"
	"strconv"
	"strings"

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
	// Sysname is the operating system name (e.g., "Linux").
	// Corresponds to `uname -s`.
	Sysname string
	// Nodename is the name within "some implementation-defined network".
	// Corresponds to `uname -n`.
	Nodename string
	// Release is the operating system release (e.g., "2.6.28").
	// Corresponds to `uname -r`.
	Release string
	// Version is the operating system version.
	// Corresponds to `uname -v`.
	Version string
	// Machine is the hardware identifier (e.g., "x86_64").
	// Corresponds to `uname -m`.
	Machine string
	// Domainname is the NIS or YP domain name. This is nonstandard GNU/Linux extension.
	Domainname string
}

// Uname is a wrapper of uname system call. See man 2 uname.
func Uname() (*Utsname, error) {
	u := &unix.Utsname{}
	if err := unix.Uname(u); err != nil {
		return nil, err
	}
	return convertUtsname(u), nil
}

func convertUtsname(u *unix.Utsname) *Utsname {
	convert := func(b []byte) string {
		return string(bytes.TrimRight(b, "\x00"))
	}
	return &Utsname{
		Sysname:    convert(u.Sysname[:]),
		Nodename:   convert(u.Nodename[:]),
		Release:    convert(u.Release[:]),
		Version:    convert(u.Version[:]),
		Machine:    convert(u.Machine[:]),
		Domainname: convert(u.Domainname[:]),
	}
}

// KernelVersion contains the Linux kernel major and minor revisions.
type KernelVersion struct {
	major, minor int
}

// Is returns true if the kernel version is major.minor else false.
func (v *KernelVersion) Is(major, minor int) bool {
	return v.major == major && v.minor == minor
}

// IsOrLater returns true if the kernel version is at least major.minor else false.
func (v *KernelVersion) IsOrLater(major, minor int) bool {
	return v.major > major || v.major == major && v.minor >= minor
}

// IsOrLess returns true if the kernel version is at most major.minor else false.
func (v *KernelVersion) IsOrLess(major, minor int) bool {
	return v.major < major || v.major == major && v.minor <= minor
}

// KernelVersionAndArch reads the Linux kernel version and arch of the system.
func KernelVersionAndArch() (*KernelVersion, string, error) {
	u, err := Uname()
	if err != nil {
		return nil, "", errors.Wrap(err, "failed to get uname")
	}
	t := strings.SplitN(u.Release, ".", 3)
	major, err := strconv.Atoi(t[0])
	if err != nil {
		return nil, "", errors.Wrapf(err, "wrong release format %q", u.Release)
	}
	minor, err := strconv.Atoi(t[1])
	if err != nil {
		return nil, "", errors.Wrapf(err, "wrong release format %q", u.Release)
	}
	return &KernelVersion{major: major, minor: minor}, u.Machine, nil
}
