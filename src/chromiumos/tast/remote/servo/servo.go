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
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
)

// Servo holds the servod connection information.
type Servo struct {
	Host string
	Port int
}

const (
	// servodDefaultHost is the default host for servod.
	servodDefaultHost = "localhost"
	// servodDefaultPort is the default port for servod.
	servodDefaultPort = 9999
	// rpcTimeout is the default and maximum timeout for XML-RPC requests to servod.
	rpcTimeout = 10 * time.Second
)

// New initializes and returns a new Servo struct.
func New(ctx context.Context, connSpec string) (*Servo, error) {
	host, port, err := parseConnSpec(connSpec)
	if err != nil {
		return nil, err
	}
	s := &Servo{host, port}

	// Ensure Servo is set up properly before returning.
	return s, s.verifyConnectivity(ctx)
}

// Default returns a new Servo struct with default values.
func Default(ctx context.Context) (*Servo, error) {
	connSpec := fmt.Sprintf("%s:%d", servodDefaultHost, servodDefaultPort)
	return New(ctx, connSpec)
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

// parseConnSpec parses a connection host+port string and returns the
// components.
func parseConnSpec(c string) (string, int, error) {
	if len(c) == 0 {
		return "", 0, errors.New("parseConnSpec got empty string")
	}

	parts := strings.Split(c, ":")
	if len(parts) == 1 {
		// If no port, return default port.
		return parts[0], servodDefaultPort, nil
	}
	if len(parts) == 2 {
		port, err := strconv.Atoi(parts[1])
		if err != nil {
			return "", 0, errors.Errorf("got invalid port int in spec %q", c)
		}
		return parts[0], port, nil
	}

	return "", 0, errors.Errorf("got invalid connection spec %q", c)
}
