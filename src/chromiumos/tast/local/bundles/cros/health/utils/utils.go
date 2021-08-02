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
	"reflect"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/crosconfig"
)

const (
	nilStr = "[nil]"
)

func ptrValueToElem(v reflect.Value) interface{} {
	if v.IsNil() {
		return nilStr
	}
	return v.Elem().Interface()
}

// PtrToElem dereferences pointer or return nilStr if nil for logging.
func PtrToElem(i interface{}) interface{} {
	return ptrValueToElem(reflect.ValueOf(i))
}

func compareError(ev, gv reflect.Value) error {
	if reflect.DeepEqual(ev.Interface(), gv.Interface()) {
		return nil
	}
	if ev.Type() != gv.Type() {
		return errors.Errorf("type doesn't match, expected: %v, got: %v", ev.Type(), gv.Type())
	}
	switch ev.Kind() {
	case reflect.Ptr:
		if ev.IsNil() || gv.IsNil() {
			return errors.Errorf("doesn't match, expected: %v, got: %v", ptrValueToElem(ev), ptrValueToElem(gv))
		}
		return compareError(ev.Elem(), gv.Elem())
	case reflect.Struct:
		for i, n := 0, ev.NumField(); i < n; i++ {
			if err := compareError(ev.Field(i), gv.Field(i)); err != nil {
				return errors.Wrap(err, ev.Type().Field(i).Name)
			}
		}
	}
	return errors.Errorf("doesn't match, expected: %+#v, got: %+#v", ev.Interface(), gv.Interface())
}

// Compare compares two interface and returns error if interface don't match.
func Compare(expected, got interface{}) error {
	if reflect.DeepEqual(expected, got) {
		return nil
	}
	err := compareError(reflect.ValueOf(expected), reflect.ValueOf(got))
	return errors.Wrap(err, "compare error")
}

// ReadFile reads a file and returns its content. Nil is returned if file not
// found.
func ReadFile(fpath string, errOut *error) *string {
	v, err := ioutil.ReadFile(fpath)
	if err != nil {
		if !os.IsNotExist(err) {
			err = errors.Wrapf(err, "failed to read file: %v", fpath)
			errOut = &err
		}
		return nil
	}
	s := strings.TrimRight(string(v), "\n")
	return &s
}

// GetCrosConfig returns the cros config from path. The dir and base name are
// extracted from the argument as the cros config path and name. Nil is returned
// if the config cannot be found.
func GetCrosConfig(ctx context.Context, cpath string, errOut *error) *string {
	v, err := crosconfig.Get(ctx, path.Dir(cpath), path.Base(cpath))
	if err != nil {
		if !crosconfig.IsNotFound(err) {
			var err error = errors.Wrapf(err, "failed to get cros config: %v", cpath)
			errOut = &err
		}
		return nil
	}
	return &v
}

// IsCrosConfigTrue returns if the value of cros config is true.
func IsCrosConfigTrue(ctx context.Context, cpath string) (bool, error) {
	var err error
	v := GetCrosConfig(ctx, cpath, &err)
	if err != nil {
		return false, nil
	}
	r := v != nil && *v == "true"
	return r, nil
}
