// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package daemonutil provides utilities for controlling background processes.
package daemonutil

import (
	"context"
	"io"

	"chromiumos/tast/errors"
)

// ReadyFunc checks the data written to ReadyWriter and returns if the service is
// ready or having error. ReadyWriter expects:
//   (false, nil) if the service is not ready yet;
//   (false, err) if the service has an error;
//   (true, nil) if the service is ready.
type ReadyFunc func([]byte) (bool, error)

// ReadyWriter stores the data written to it and identifies if a service is ready
// or already failed.
type ReadyWriter struct {
	buf   []byte
	ch    chan error
	done  bool
	ready ReadyFunc
}

var _ io.WriteCloser = (*ReadyWriter)(nil)

// NewReadyWriter creates a ReadyWriter object with f to detect the state of the
// service.
func NewReadyWriter(f ReadyFunc) *ReadyWriter {
	return &ReadyWriter{
		ch:    make(chan error, 1),
		ready: f,
	}
}

// Write writes p to the buffer to detect the ready/error event of the service.
// It implements io.Writer interface.
func (w *ReadyWriter) Write(p []byte) (int, error) {
	if w.done {
		return len(p), nil
	}
	w.buf = append(w.buf, p...)
	if ok, err := w.ready(w.buf); err != nil {
		w.ch <- err
		close(w.ch)
		w.buf = nil
		w.done = true
	} else if ok {
		w.ch <- nil
		close(w.ch)
		w.buf = nil
		w.done = true
	}
	// The service is not yet ready and no error detected.
	// Have the data buffered in w.buf and return success. Wait upcoming
	// Write for more data to determine the state of the service.
	return len(p), nil
}

// Close closes the writer and emits error if it has not yet detected ready/error
// event. It implements io.Closer interface.
func (w *ReadyWriter) Close() error {
	if w.done {
		return nil
	}
	w.done = true
	w.buf = nil
	w.ch <- errors.New("service exited unexpectedly")
	close(w.ch)
	return nil
}

// Wait waits until the service is ready or some errors happens.
func (w *ReadyWriter) Wait(ctx context.Context) error {
	select {
	case err := <-w.ch:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
