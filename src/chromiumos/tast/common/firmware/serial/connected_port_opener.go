// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package serial

import (
	"context"
	"time"

	tarm "github.com/tarm/serial"
)

// Config holds parameters of the serial port.
type Config struct {
	// Name is the path on the filesystem to the port.
	Name string
	// Baud rate of the port.
	Baud int
	// ReadTimeout is the max expected duration of silence during reads.
	ReadTimeout time.Duration
	// Default Parity is none, Stop bits is 1, Data bits is 8.
	// Add here and pipe these through in OpenPort if needed.
}

// ConnectedPortOpener opens a directly connected port on the localhost.  In a
// dut context during local bundle execution, the localhost is the dut.
type ConnectedPortOpener struct {
	Config
}

// OpenPort opens the port.
func (c *ConnectedPortOpener) OpenPort(ctx context.Context) (Port, error) {
	tarmCfg := &tarm.Config{Name: c.Name, Baud: c.Baud, ReadTimeout: c.ReadTimeout}

	p, err := tarm.OpenPort(tarmCfg)
	if err != nil {
		return nil, err
	}
	return &ConnectedPort{p}, nil
}

// NewConnectedPortOpener creates a new ConnectedPortOpener.
func NewConnectedPortOpener(name string, baud int, readTimeout time.Duration) *ConnectedPortOpener {
	cfg := Config{name, baud, readTimeout}
	return &ConnectedPortOpener{Config: cfg}
}
