// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package drivefs

import (
	"strconv"

	"golang.org/x/sys/unix"
)

// GetXattrBytes gets the xattr `name` of `path` as a byte array.
func GetXattrBytes(path, name string) ([]byte, error) {
	size, err := unix.Getxattr(path, name, nil)
	if err != nil {
		return nil, err
	}
	data := make([]byte, size)
	if size == 0 {
		return data, nil
	}
	size, err = unix.Getxattr(path, name, data)
	if err != nil {
		return nil, err
	}
	return data[0:size], nil
}

// GetXattrString gets the xattr `name` of `path` as a string.
func GetXattrString(path, name string) (string, error) {
	data, err := GetXattrBytes(path, name)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// GetXattrBool gets the xattr `name` of `path` as a bool.
func GetXattrBool(path, name string) (bool, error) {
	data, err := GetXattrString(path, name)
	if err != nil {
		return false, err
	}
	// DriveFS only returns "1" and "0", which `ParseBool` supports.
	return strconv.ParseBool(data)
}

// SetXattrBytes sets the xattr `name` of `path` to `data` as a byte array.
func SetXattrBytes(path, name string, data []byte) error {
	return unix.Setxattr(path, name, data, 0)
}

// SetXattrString sets the xattr `name` of `path` to `data` as a string.
func SetXattrString(path, name, data string) error {
	return SetXattrBytes(path, name, []byte(data))
}

// SetXattrBool sets the xattr `name` of `path` to `data` as a bool.
func SetXattrBool(path, name string, data bool) error {
	if data {
		return SetXattrString(path, name, "1")
	}
	return SetXattrString(path, name, "0")
}
