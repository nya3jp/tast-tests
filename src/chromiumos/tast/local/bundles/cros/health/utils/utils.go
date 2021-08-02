// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package utils provides util functions for health tast.
package utils

import (
	"context"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/crosconfig"
)

// PtrToStr converts pointer of string to string for logging.
func PtrToStr(s *string) string {
	if s == nil {
		return "[Null]"
	}
	return *s
}

func compareError(expected, got string) error {
	return errors.Errorf("doesn't match, expected: %v, got: %v", expected, got)
}

// CompareString compares two string and returns error if strings don't match.
func CompareString(expected, got string) error {
	if expected != got {
		return compareError(expected, got)
	}
	return nil
}

// CompareStringPtr compares two pointer of string. If the pointer is not nil,
// the contents is compared.
func CompareStringPtr(expected, got *string) error {
	if expected == nil && got == nil {
		return nil
	}
	if expected == nil || got == nil || *expected != *got {
		return compareError(PtrToStr(expected), PtrToStr(got))
	}
	return nil
}

// GetCrosConfig returns the cros config from path. The dir and base name are
// extracted from the argument as the cros config path and name. Nil is returned
// if the config cannot be found.
func GetCrosConfig(ctx context.Context, cpath string) (*string, error) {
	v, err := crosconfig.Get(ctx, path.Dir(cpath), path.Base(cpath))
	if err != nil {
		if !crosconfig.IsNotFound(err) {
			return nil, errors.Wrapf(err, "failed to get cros config: %v", cpath)
		}
		return nil, nil
	}
	return &v, nil
}

// IsCrosConfigTrue returns if the value of cros config is true.
func IsCrosConfigTrue(ctx context.Context, cpath string) (bool, error) {
	v, err := GetCrosConfig(ctx, cpath)
	if err != nil {
		return false, nil
	}
	r := v != nil && *v == "true"
	return r, nil
}

// ReadFile reads a file and returns its content. Nil is returned if file not
// found.
func ReadFile(fpath string) (*string, error) {
	v, err := ioutil.ReadFile(fpath)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, errors.Wrapf(err, "failed to read file: %v", fpath)
		}
		return nil, nil
	}
	s := strings.TrimRight(string(v), "\n")
	return &s, nil
}
