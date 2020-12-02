// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package fake implements a fake network printer reading LPR protocol.
package fake

import (
	"context"
	"io/ioutil"
	"net"
	"sync/atomic"

	"chromiumos/tast/errors"
)

// Printer is a fake printer implementation, which reads LPR requests,
// and returns them via ReadRequest.
type Printer struct {
	ln    net.Listener
	ch    chan []byte
	conn  net.Conn
	state int32
}

// NewPrinter creates and starts a fake printer.
func NewPrinter(ctx context.Context) (*Printer, error) {
	const address = "localhost:9100"
	ln, err := net.Listen("tcp", address)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to listen on %s", address)
	}
	p := &Printer{ln: ln, ch: make(chan []byte, 1)}
	go p.run()
	return p, nil
}

// run runs the background task to read the LPR requests and to proxy them
// to ch.
func (p *Printer) run() {
	defer close(p.ch)
	conn, err := p.ln.Accept()
	if err != nil {
		return
	}
	p.conn = conn
	if atomic.AddInt32(&p.state, 1) > 1 {
		conn.Close()
		return
	}
	data, err := ioutil.ReadAll(conn)
	if err != nil {
		return
	}
	p.ch <- data
}

// Close stops the fake printer.
func (p *Printer) Close() {
	// This triggers to return an error by Accept() in run(). So,
	// eventually the goroutine exits.
	p.ln.Close()
	// This triggers ioutil.ReadAll() in run() to return an error.
	if atomic.AddInt32(&p.state, 1) > 1 {
		p.conn.Close()
	}
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
