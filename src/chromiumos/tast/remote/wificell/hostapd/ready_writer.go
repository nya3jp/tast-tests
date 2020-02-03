// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hostapd

import (
	"bytes"
	"context"

	"chromiumos/tast/errors"
)

// readyWriter parses stdout of hostapd and identifies if it is ready or
// the daemon failed.
type readyWriter struct {
	buf  []byte
	ch   chan error
	done bool
}

func newReadyWriter() *readyWriter {
	return &readyWriter{
		ch: make(chan error, 1),
	}
}

// Write function implements io.Writer interface. It writes the data to
// buffer for readyWriter to detect hostapd's ready/error event.
func (w *readyWriter) Write(p []byte) (int, error) {
	if w.done {
		return len(p), nil
	}
	w.buf = append(w.buf, p...)
	if bytes.Contains(w.buf, []byte("Interface initialization failed")) {
		w.ch <- errors.New("hostapd failed to initialize AP interface")
		close(w.ch)
		w.buf = nil
		w.done = true
	} else if bytes.Contains(w.buf, []byte("Setup of interface done")) {
		w.ch <- nil
		close(w.ch)
		w.buf = nil
		w.done = true
	}
	return len(p), nil
}

// Close function implements io.Closer interface. It closes the writer
// and emit error if it has not yet detected ready/error event.
func (w *readyWriter) Close() error {
	if w.done {
		return nil
	}
	w.done = true
	w.buf = nil
	w.ch <- errors.New("hostapd exited unexpectedly")
	close(w.ch)
	return nil
}

// wait until hostapd is ready or some errors happens.
func (w *readyWriter) wait(ctx context.Context) error {
	select {
	case err := <-w.ch:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
