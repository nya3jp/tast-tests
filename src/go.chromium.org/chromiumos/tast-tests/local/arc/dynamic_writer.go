// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"io"
	"sync"
)

// dynamicWriter synchronously writes to a dynamically-configurable io.Writer.
// It is safe to call from multiple goroutines.
type dynamicWriter struct {
	mutex sync.Mutex // protects w
	w     io.Writer  // destination for Write calls
}

// Write implements io.Writer.
func (dw *dynamicWriter) Write(p []byte) (n int, err error) {
	dw.mutex.Lock()
	defer dw.mutex.Unlock()

	if dw.w == nil {
		return len(p), nil
	}
	return dw.w.Write(p)
}

// setDest directs all future writes to w, which may be nil.
func (dw *dynamicWriter) setDest(w io.Writer) {
	dw.mutex.Lock()
	defer dw.mutex.Unlock()
	dw.w = w
}
