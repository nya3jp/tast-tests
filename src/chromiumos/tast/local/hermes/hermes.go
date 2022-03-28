// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package hermes provides D-Bus wrappers and utilities for Hermes.
// https://chromium.googlesource.com/chromiumos/platform2/+/HEAD/hermes/README.md
package hermes

import (
	"context"
	"reflect"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
)

// CheckProperty reads a DBus property on a DBusObject. Returns an error if the value does not match the expected value
func CheckProperty(ctx context.Context, o *dbusutil.DBusObject, prop string, expected interface{}) error {
	var actual interface{}
	if err := o.Get(ctx, prop, &actual); err != nil {
		return errors.Wrap(err, "failed to check property")
	}
	if reflect.TypeOf(actual) != reflect.TypeOf(expected) {
		return errors.Errorf("unexpected type for %s, got: %T, want: %T", prop, actual, expected)
	}
	if actual != expected {
		return errors.Errorf("unexpected %s, got: %v, want: %v", prop, actual, expected)
	}

	return nil
}
