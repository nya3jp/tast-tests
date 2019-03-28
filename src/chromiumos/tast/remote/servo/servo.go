// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package servo is used to communicate with servo devices connected to DUTs.
// It communicates with servod over XML-RPC.
// More details on servo: https://www.chromium.org/chromium-os/servo
package servo

import (
	"context"
	"fmt"
	"net"
	"time"

	"chromiumos/tast/errors"
)

// Servo holds the servod connection information.
type Servo struct {
	host string
	port string
}

const (
	// servodDefaultHost is the default host for servod.
	servodDefaultHost = "localhost"
	// servodDefaultPort is the default port for servod.
	servodDefaultPort = "9999"
	// rpcTimeout is the default and maximum timeout for XML-RPC requests to servod.
	rpcTimeout = 10 * time.Second
)

// NewServo creates a new Servo object for communicating with a servod instance.
// connSpec holds servod's location, either as "host:port" or just "host"
// (to use the default port).
func NewServo(ctx context.Context, connSpec string) (*Servo, error) {
	specHost, specPort, err := net.SplitHostPort(connSpec)
	if err != nil {
		return nil, err
	}
	// TODO(CL): Test these!
	if specHost == "" {
		specHost = servodDefaultHost
	}
	if specPort == "" {
		specPort = servodDefaultPort
	}
	s := &Servo{specHost, specPort}

	// Ensure Servo is set up properly before returning.
	return s, s.verifyConnectivity(ctx)
}

// Default creates a Servo object for communicating with a local servod
// instance using the default port.
func Default(ctx context.Context) (*Servo, error) {
	connSpec := fmt.Sprintf("%s:%d", servodDefaultHost, servodDefaultPort)
	return NewServo(ctx, connSpec)
}

// verifyConnectivity sends and verifies an echo request to make sure
// everything is set up properly.
func (s *Servo) verifyConnectivity(ctx context.Context) error {
	const msg = "hello from servo"
	actualMessage, err := s.Echo(ctx, "hello from servo")
	if err != nil {
		return err
	}

	const expectedMessage = "ECH0ING: " + msg
	if actualMessage != expectedMessage {
		return errors.Errorf("echo verification request returned %q; expected %q", actualMessage, expectedMessage)
	}

	return nil
}
