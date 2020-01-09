// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chrome

import (
	"context"
	"time"

	"github.com/mafredri/cdp/protocol/runtime"

	"chromiumos/tast/errors"
)

// JSObject is a reference to a JavaScript object.
// JSObjects must be released or they will stop the JavaScript GC from freeing the memory they reference.
type JSObject struct {
	Conn *Conn
	ro   runtime.RemoteObject
}

// Call calls the given JavaScript function this Object.
// The passed arguments must be JSObjects or be able to marshal to JSON.
// The JavaScript function may incorrectly bind the remote object if written with arrow syntax.
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
		out = &newOb.ro
	}

	repl, err := ob.Conn.co.CallOn(ctx, *ob.ro.ObjectID, out, fn, callArgs...)
	if err != nil {
		if repl != nil && repl.ExceptionDetails != nil {
			ob.Conn.lw.Report(time.Now(), "callon-error", err.Error(), repl.ExceptionDetails.StackTrace)
		}
		return err
	}
	if returnJSObject {
		newOb.Conn = ob.Conn
	}
	return nil
}

// Release releases this object's reference to JavaScript.
func (ob *JSObject) Release(ctx context.Context) error {
	return ob.Conn.co.ReleaseObject(ctx, ob.ro)
}
