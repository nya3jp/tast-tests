// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chrome

import (
	"context"

	"github.com/mafredri/cdp/protocol/runtime"

	"chromiumos/tast/errors"
)

// JSObject is a reference to a JavaScript object.
type JSObject struct {
	conn *Conn
	ro   runtime.RemoteObject
}

// Call calls the given JavaScript function this Object.
// The passed arguments must be able to marshal to JSON.
// The JavaScript function may incorrectly bind the remote object if written with arrow syntax.
// If out is given, the returned value is set.
// In case of JavaScript exceptions, an error is return.
func (ob *JSObject) Call(ctx context.Context, out interface{}, fn string, args ...interface{}) error {
	// Convert JSObject parameters to RemoteObject for cdputils CallOn.
	var callArgs []interface{}
	for _, arg := range args {
		switch v := arg.(type) {
		case *JSObject:
			if v.ro.ObjectID == nil {
				return errors.New("invalid javascript object as argument")
			}
			callArgs = append(callArgs, v.ro)
		case JSObject:
			if v.ro.ObjectID == nil {
				return errors.New("invalid javascript object as argument")
			}
			callArgs = append(callArgs, v.ro)
		default:
			callArgs = append(callArgs, v)
		}
	}

	// Check if returning JSObject
	newOb, returnJSObject := out.(*JSObject)
	if returnJSObject {
		newOb.conn = ob.conn
		out = &newOb.ro
	}

	_, err := ob.conn.co.CallOn(ctx, *ob.ro.ObjectID, out, fn, callArgs...)
	return err
}

// Release releases this object's reference to JavaScript.
func (ob *JSObject) Release(ctx context.Context) error {
	return ob.conn.co.ReleaseObject(ctx, ob.ro)
}
