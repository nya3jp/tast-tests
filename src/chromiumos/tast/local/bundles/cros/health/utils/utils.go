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

// For mocking
var readFile = ioutil.ReadFile

// ReadStringFile reads a file and returns its content as string.
func ReadStringFile(fpath string) (string, error) {
	v, err := readFile(fpath)
	if err != nil {
		return "", errors.Wrapf(err, "failed to read file: %v", fpath)
	}
	return strings.TrimRight(string(v), "\n"), nil
}

// ReadOptionalStringFile returns nil if file not found.
func ReadOptionalStringFile(fpath string) (*string, error) {
	v, err := ReadStringFile(fpath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	return &v, nil
}

// For mocking
var getCrosConfig = crosconfig.Get

// GetCrosConfig returns the cros config from path. The dir and base name are
// extracted from the argument as the cros config path and name.
func GetCrosConfig(ctx context.Context, cpath string) (string, error) {
	v, err := getCrosConfig(ctx, path.Dir(cpath), path.Base(cpath))
	if err != nil {
		return "", errors.Wrapf(err, "failed to get cros config: %v", cpath)
	}
	return v, nil
}

// GetOptionalCrosConfig returns nil if the config cannot be found.
func GetOptionalCrosConfig(ctx context.Context, cpath string) (*string, error) {
	v, err := GetCrosConfig(ctx, cpath)
	if err != nil {
		var e *crosconfig.ErrNotFound
		if errors.As(err, &e) {
			return nil, nil
		}
		return nil, err
	}
	return &v, nil
}

// IsCrosConfigTrue returns if the value of cros config is true.
func IsCrosConfigTrue(ctx context.Context, cpath string) (bool, error) {
	v, err := GetOptionalCrosConfig(ctx, cpath)
	if err != nil {
		return false, err
	}
	r := v != nil && *v == "true"
	return r, nil
}
