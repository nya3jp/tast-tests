// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package fake implements a fake printer.
package fake

import (
	"context"
	"io/ioutil"
	"net"

	"chromiumos/tast/errors"
)

// Printer is a fake printer implementation, which reads requests, and returns
// it via ReadRequest.
type Printer struct {
	ln net.Listener
	ch chan []byte
}

// NewPrinter creates and starts a fake printer.
func NewPrinter() (*Printer, error) {
	ln, err := net.Listen("tcp", "localhost:9100")
	if err != nil {
		return nil, errors.Wrap(err, "Failed to listen localhost:9100")
	}

	p := &Printer{ln, make(chan []byte)}
	go func() {
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
	}()

	return p, nil
}

// Close stops the fake printer.
func (p *Printer) Close() {
	p.ln.Close()
}

// ReadRequest returns the print request.
func (p *Printer) ReadRequest(ctx context.Context) []byte {
	select {
	case <-ctx.Done():
		return nil
	case v := <-p.ch:
		return v
	}
}
