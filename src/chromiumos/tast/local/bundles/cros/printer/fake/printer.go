// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package fake implements a fake network printer reading LPR protocol.
package fake

import (
	"context"
	"io/ioutil"
	"net"

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
		return nil, errors.Wrapf(err, "failed to listen on: %s", address)
	}

	p := &Printer{ln, make(chan []byte)}
	go p.run()
	return p, nil
}

// run runs the background task to read the LPR requests and to proxy them
// to ch.
func (p *Printer) run() {
	for {
		func() {
			conn, err := p.ln.Accept()
			if err != nil {
				return
			}
			defer conn.Close()
			data, err := ioutil.ReadAll(conn)
			if err != nil {
				return
			}
			p.ch <- data
		}()
	}
}

// Close stops the fake printer.
func (p *Printer) Close() {
	p.ln.Close()
}

// ReadRequest returns the print request.
func (p *Printer) ReadRequest(ctx context.Context) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, errors.New("ReadRequest timed out")
	case v := <-p.ch:
		return v, nil
	}
}
