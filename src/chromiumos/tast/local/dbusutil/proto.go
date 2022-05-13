// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dbusutil

import (
	"context"

	"github.com/godbus/dbus/v5"
	"github.com/golang/protobuf/proto"

	"chromiumos/tast/errors"
)

// CallProtoMethodWithSequence marshals in, passes it as a byte array arg to method on obj,
// and unmarshals a byte array arg from the response to out. The sequence number indicating
// the order of the method response on the DBus connection is returned.
//
// This sequence number may be correlated with the sequence number of other method calls
// and signals on the same DBus connection, such as to implement a race-free subscribe and
// get-current-state operation.
//
// The method specified should be prefixed by a D-Bus interface name.
// Both in and out may be nil.
func CallProtoMethodWithSequence(ctx context.Context, obj dbus.BusObject, method string, in, out proto.Message) (dbus.Sequence, error) {
	var args []interface{}
	if in != nil {
		marshIn, err := proto.Marshal(in)
		if err != nil {
			return 0, errors.Wrapf(err, "failed marshaling %s arg", method)
		}
		args = append(args, marshIn)
	}

	call := obj.CallWithContext(ctx, method, 0, args...)
	if call.Err != nil {
		return call.ResponseSequence, errors.Wrapf(call.Err, "failed calling %s", method)
	}
	if out != nil {
		var marshOut []byte
		if err := call.Store(&marshOut); err != nil {
			return call.ResponseSequence, errors.Wrapf(err, "failed reading %s response", method)
		}
		if err := proto.Unmarshal(marshOut, out); err != nil {
			return call.ResponseSequence, errors.Wrapf(err, "failed unmarshaling %s response", method)
		}
	}
	return call.ResponseSequence, nil
}

// CallProtoMethod marshals in, passes it as a byte array arg to method on obj,
// and unmarshals a byte array arg from the response to out. method should be prefixed
// by a D-Bus interface name. Both in and out may be nil.
func CallProtoMethod(ctx context.Context, obj dbus.BusObject, method string, in, out proto.Message) error {
	_, err := CallProtoMethodWithSequence(ctx, obj, method, in, out)
	return err
}
