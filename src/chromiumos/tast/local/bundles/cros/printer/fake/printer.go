// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package fake implements a fake network printer reading LPR protocol.
package fake

import (
	"context"
	"io/ioutil"
	"net"
	"time"

	"chromiumos/tast/errors"
)

// Printer is a fake printer implementation, which reads LPR requests,
// and returns them via ReadRequest.
type Printer struct {
	ln net.Listener
	ch chan []byte
}

// NewPrinter creates and starts a fake printer.
func NewPrinter() (*Printer, error) {
	const address = "localhost:9100"
	ln, err := net.Listen("tcp", address)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to listen on %s", address)
	}

	p := &Printer{ln, make(chan []byte)}
	go p.run()
	return p, nil
}

// run runs the background task to read the LPR requests and to proxy them
// to ch.
func (p *Printer) run() {
	for {
		if eof := func() bool {
			conn, err := p.ln.Accept()
			if err != nil {
				return true
			}
			defer conn.Close()
			// Make sure that the goroutine exits eventually even
			// if the process connected to |conn| is hanging.
			conn.SetReadDeadline(time.Now().Add(2 * time.Second))
			data, err := ioutil.ReadAll(conn)
			if err != nil {
				return false
			}
			p.ch <- data
			return false
		}(); eof {
			close(p.ch)
			return
		}
	}
}

// Close stops the fake printer.
func (p *Printer) Close(ctx context.Context) error {
	p.ln.Close()
	// Read and discard all remaining messages.
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case _, ok := <-p.ch:
			if !ok {
				return nil
			}
		}
	}
}

// ReadRequest returns the print request.
func (p *Printer) ReadRequest(ctx context.Context) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, errors.Wrap(ctx.Err(), "ReadRequest timed out")
	case v := <-p.ch:
		return v, nil
	}
}
