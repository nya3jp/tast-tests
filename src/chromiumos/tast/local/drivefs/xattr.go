// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package drivefs

import (
	"strconv"

	"golang.org/x/sys/unix"

	"chromiumos/tast/errors"
)

// GetXattr gets the xattr `name` of `path` and places it in `value`.
func GetXattr(path, name string, value interface{}) error {
	size, err := unix.Getxattr(path, name, nil)
	if err != nil {
		return err
	}
	data := make([]byte, size)
	if size == 0 {
		return nil
	}
	size, err = unix.Getxattr(path, name, data)
	if err != nil {
		return err
	}
	switch ptr := value.(type) {
	case *bool:
		*ptr, err = strconv.ParseBool(string(data))
		if err != nil {
			return err
		}
	case *string:
		*ptr = string(data)
	case *[]byte:
		*ptr = data
	default:
		return errors.New("unknown xattr data type supplied")
	}
	return nil
}

// SetXattr sets the xattr `name` of `path` to `value`.
func SetXattr(path, name string, value interface{}) error {
	switch xattr := value.(type) {
	case []byte:
		return unix.Setxattr(path, name, xattr, 0)
	case string:
		return unix.Setxattr(path, name, []byte(xattr), 0)
	case bool:
		if xattr {
			return unix.Setxattr(path, name, []byte("1"), 0)
		}
		return unix.Setxattr(path, name, []byte("0"), 0)
	default:
		return errors.New("unknown xattr data type supplied")
	}
}
