// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifiutil

// Destructor Stack mimics language's defer() mechanics with the main difference
// that its destructor can be exported outside the function where it was created
// to be called at a later time (e.g. deferred).
// If it's not exported, it will trigger all defers when function's stack is unwound.
//
// Limitations:
// (1) Do not use one stack for different methods, it's going to be messy,
// especially if you play with contexts and their cancellations.
// (2) Be careful when using local variables in deferred functions. Treat
// them like they would be evaluated at the end of function.

// destructorStack holds the stack of functions to be called.
type destructorStack struct {
	stack    []func()
	exported bool
}

// newDestructorStack returns the stack and its (conditional) destructor
func newDestructorStack() (*destructorStack, func()) {
	ds := &destructorStack{exported: false}
	return ds, ds.destroyIfNotExported
}

// push a function to be deferred.
func (ds *destructorStack) push(f func()) {
	ds.stack = append(ds.stack, f)
}

// destroy is an unconditional destructor.
func (ds *destructorStack) destroy() {
	for stackLen := (len(ds.stack)); stackLen > 0; stackLen-- {
		idx := stackLen - 1
		ds.stack[idx]()
		ds.stack = ds.stack[:idx]
	}
}

// destroyIfNotExported is a conditional destructor - it will trigger
// destruction if the stack was not exported.
func (ds *destructorStack) destroyIfNotExported() {
	if !ds.exported {
		ds.destroy()
	}
}

// export the stack outside the function body.
func (ds *destructorStack) export() *destructorStack {
	ds.exported = true
	return ds
}
