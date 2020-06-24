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
	"chromiumos/tast/testing"
)

// Servo holds the servod connection information.
type Servo struct {
	host string
	port int

	// If initialV4Role is set, then upon Servo.Close(), the V4Role control will be set to initialV4Role.
	initialV4Role V4RoleValue
}

const (
	// servodDefaultHost is the default host for servod.
	servodDefaultHost = "localhost"
	// servodDefaultPort is the default port for servod.
	servodDefaultPort = 9999
	// rpcTimeout is the default and maximum timeout for XML-RPC requests to servod.
	rpcTimeout = 10 * time.Second
)

// New creates a new Servo object for communicating with a servod instance.
// connSpec holds servod's location, either as "host:port" or just "host"
// (to use the default port).
func New(ctx context.Context, connSpec string) (*Servo, error) {
	host, port, err := parseConnSpec(connSpec)
	if err != nil {
		return nil, err
	}
	s := &Servo{host: host, port: port}

	// Ensure Servo is set up properly before returning.
	return s, s.verifyConnectivity(ctx)
}

// Default creates a Servo object for communicating with a local servod
// instance using the default port.
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

// parseConnSpec parses a connection host:port string and returns the
// components.
func parseConnSpec(c string) (host string, port int, err error) {
	if len(c) == 0 {
		return "", 0, errors.New("got empty string")
	}

	parts := strings.Split(c, ":")
	if len(parts) == 1 {
		// If no port, return default port.
		return parts[0], servodDefaultPort, nil
	}
	if len(parts) == 2 {
		port, err = strconv.Atoi(parts[1])
		if err != nil {
			return "", 0, errors.Errorf("got invalid port int in spec %q", c)
		}
		return parts[0], port, nil
	}

	return "", 0, errors.Errorf("got invalid connection spec %q", c)
}

// Close performs Servo cleanup.
func (s *Servo) Close(ctx context.Context) error {
	if s.initialV4Role != "" {
		testing.ContextLogf(ctx, "Restoring %q to %q", V4Role, s.initialV4Role)
		if err := s.SetV4Role(ctx, s.initialV4Role); err != nil {
			return errors.Wrapf(err, "restoring servo control %q to %q", V4Role, s.initialV4Role)
		}
	}
	return nil
}
