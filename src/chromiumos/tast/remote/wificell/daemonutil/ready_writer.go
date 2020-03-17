// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package daemonutil provides utilities for controlling background processes.
package daemonutil

import (
	"context"

	"chromiumos/tast/errors"
)

// ReadyFunc checks the data written to ReadyWriter and returns if the service is
// ready or having error. ReadyWriter expects:
//   If the service is not yet ready, it should return (false, nil).
//   If the service has error, it should return (false, non-nil error).
//   If the service is ready, it should return (true, nil).
type ReadyFunc func([]byte) (bool, error)

// ReadyWriter stores the data written to it and identifies if a service is ready
// or already failed.
type ReadyWriter struct {
	buf       []byte
	ch        chan error
	done      bool
	readyFunc ReadyFunc
}

// NewReadyWriter creates a ReadyWriter object with f to detect the state of the
// service.
func NewReadyWriter(f ReadyFunc) *ReadyWriter {
	return &ReadyWriter{
		ch:        make(chan error, 1),
		readyFunc: f,
	}
}

// Write writes p to the buffer to detect the ready/error event of the service.
// It implements io.Writer interface.
func (w *ReadyWriter) Write(p []byte) (int, error) {
	if w.done {
		return len(p), nil
	}
	w.buf = append(w.buf, p...)
	if ok, err := w.readyFunc(w.buf); ok {
		w.ch <- nil
		close(w.ch)
		w.buf = nil
		w.done = true
	} else if err != nil {
		w.ch <- err
		close(w.ch)
		w.buf = nil
		w.done = true
	}
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
