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
	ln     net.Listener
	cancel func()
	ch     chan []byte
}

// NewPrinter creates and starts a fake printer.
func NewPrinter(ctx context.Context) (*Printer, error) {
	const address = "localhost:9100"
	ln, err := net.Listen("tcp", address)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to listen on %s", address)
	}

	rctx, cancel := context.WithCancel(ctx)
	p := &Printer{ln, cancel, make(chan []byte)}
	go p.run(rctx)
	return p, nil
}

// run runs the background task to read the LPR requests and to proxy them
// to ch.
func (p *Printer) run(ctx context.Context) {
	for {
		if eof := func() bool {
			conn, err := p.ln.Accept()
			if err != nil {
				return true
			}
			defer conn.Close()
			// Make sure that the goroutine exits eventually even
			// if the process connected to |conn| is hanging.
			rctx, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()
			deadline, _ := rctx.Deadline()
			conn.SetReadDeadline(deadline)
			data, err := ioutil.ReadAll(conn)
			if err != nil {
				return false
			}
			select {
			case <-ctx.Done():
				// If timed out, we should see EOF on a
				// following iteration and exit then.
			case p.ch <- data:
			}
			return false
		}(); eof {
			close(p.ch)
			return
		}
	}
}

// Close stops the fake printer.
func (p *Printer) Close() {
	// This triggers to return an error by Accept() in run(). So,
	// eventually the goroutine exits.
	p.ln.Close()
	// Also, call the cancel to notify the context used in run().
	p.cancel()
}

// ReadRequest returns the print request.
func (p *Printer) ReadRequest(ctx context.Context) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, errors.Wrap(ctx.Err(), "ReadRequest timed out")
	case v, ok := <-p.ch:
		if !ok {
			return nil, errors.New("p.ch is unexpectedly closed")
		}
		return v, nil
	}
}
