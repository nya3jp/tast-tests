// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package servo is used to communicate with servo devices connected to DUTs.
// It communicates with servod over XML-RPC.
// More details on servo: https://www.chromium.org/chromium-os/servo
//
// Caution: If you reboot the ChromeOS EC:
// - If using a CCD servo, you should call WatchdogRemove(ctx, CCD) or servod will fail.
// - Use Helper.WaitConnect instead of DUT.WaitConnect.
package servo

import (
	"context"

	"chromiumos/tast/common/xmlrpc"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// Servo holds the servod connection information.
type Servo struct {
	xmlrpc *xmlrpc.XMLRpc

	// Cache queried attributes that won't change.
	version       string
	dutConnType   DUTConnTypeValue
	servoType     string
	hasCCD        bool
	hasServoMicro bool
	hasC2D2       bool
	isDualV4      bool

	// If initialPDRole is set, then upon Servo.Close(), the PDRole control will be set to initialPDRole.
	initialPDRole PDRoleValue

	removedWatchdogs []WatchdogValue
}

const (
	// servodDefaultHost is the default host for servod.
	servodDefaultHost = "localhost"
	// servodDefaultPort is the default port for servod.
	servodDefaultPort = 9999
)

// New creates a new Servo object for communicating with a servod instance.
// connSpec holds servod's location, either as "host:port" or just "host"
// (to use the default port).
func New(ctx context.Context, host string, port int) (*Servo, error) {
	s := &Servo{xmlrpc: xmlrpc.New(host, port)}

	// Ensure Servo is set up properly before returning.
	return s, s.verifyConnectivity(ctx)
}

// Default creates a Servo object for communicating with a local servod
// instance using the default port.
func Default(ctx context.Context) (*Servo, error) {
	return New(ctx, servodDefaultHost, servodDefaultPort)
}

// NewDirect returns a servo object for communication with a servod instance running at spec.
// During local tests, instead of establishing a ssh proxy with servo host, creating a direct
// connection is preferred. Param spec is either "host:port" or just "host" to use default port.
// Please make sure "host" is the address reachable from DUT.
func NewDirect(ctx context.Context, spec string) (*Servo, error) {
	host, port, _, err := splitHostPort(spec)
	if err != nil {
		return nil, err
	}
	return New(ctx, host, port)
}

func (s *Servo) reconnect(ctx context.Context, host string, port int) error {
	s.xmlrpc = xmlrpc.New(host, port)
	return s.verifyConnectivity(ctx)
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

// Close performs Servo cleanup.
func (s *Servo) Close(ctx context.Context) error {
	var firstError error
	if s.initialPDRole != "" && s.initialPDRole != PDRoleNA {
		testing.ContextLogf(ctx, "Restoring %q to %q", PDRole, s.initialPDRole)
		if err := s.SetPDRole(ctx, s.initialPDRole); err != nil && firstError == nil {
			firstError = errors.Wrapf(err, "restoring servo control %q to %q", PDRole, s.initialPDRole)
		}
	}
	for _, v := range s.removedWatchdogs {
		testing.ContextLogf(ctx, "Restoring servo watchdog %q", v)
		if err := s.WatchdogAdd(ctx, v); err != nil && firstError == nil {
			firstError = errors.Wrapf(err, "restoring watchdog %q", v)
		}
	}
	return firstError
}
