// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package driver

import (
	"context"
	"time"

	"github.com/mafredri/cdp/protocol/runtime"

	"chromiumos/tast/errors"
)

// JSObject is a reference to a JavaScript object.
// JSObjects must be released or they will stop the JavaScript GC from freeing the memory they reference.
type JSObject struct {
	conn *Conn
	ro   runtime.RemoteObject
}

// Call calls the given JavaScript function this Object.
// The passed arguments must be of type *JSObject or be able to marshal to JSON.
// If fn is an arrow function, the "this" in the function body will be the window object instead of
// the object referred to by JSObject ob, and that will probably lead to unintended behavior.
// If out is given, the returned value is set.
// If out is a *chrome.JSObject, a reference to the result is returned.
// The *chrome.JSObject should get released or the memory it references will not be freed.
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
			callArgs = append(callArgs, &v.ro)
		case JSObject:
			return errors.New("JSObject not supported as an argument; please use *JSObject")
		default:
			callArgs = append(callArgs, v)
		}
	}

	// Check if returning JSObject
	newOb, returnJSObject := out.(*JSObject)
	if returnJSObject {
		out = &newOb.ro
	}

	exc, err := ob.conn.co.CallOn(ctx, *ob.ro.ObjectID, out, fn, callArgs...)
	if err != nil {
		if exc != nil {
			ob.conn.lw.Report(time.Now(), "callon-error", err.Error(), exc.StackTrace)
		}
		return err
	}
	if returnJSObject {
		newOb.conn = ob.conn
	}
	return nil
}

// Release releases this object's reference to JavaScript.
func (ob *JSObject) Release(ctx context.Context) error {
	return ob.conn.co.ReleaseObject(ctx, ob.ro)
}
