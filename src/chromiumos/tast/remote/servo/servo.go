// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package servo is used to communicate with servo devices connected to DUTs.
// It communicates with servod over XML-RPC.
// More details on servo: https://www.chromium.org/chromium-os/servo
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

	removedWatchdogs map[WatchdogValue]bool
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
	s.removedWatchdogs = make(map[WatchdogValue]bool)

	// Ensure Servo is set up properly before returning.
	return s, s.VerifyConnectivity(ctx, "")
}

// Default creates a Servo object for communicating with a local servod
// instance using the default port.
func Default(ctx context.Context) (*Servo, error) {
	return New(ctx, servodDefaultHost, servodDefaultPort)
}

// VerifyConnectivity sends an echo request and verifies its response
// in order to verify the servo connection is working.
//
// The id argument corresponds to message that will be sent to servod
// and echoed back. This can be left blank, but it is reccomended to
// specify something specific to the test, like the test's name.
// This is because the echo message will appear in servod's log and
// may help in tracking down servo issues.
//
// Note that this does not verify that servod's devices (like ccd) are
// still connected.
func (s *Servo) VerifyConnectivity(ctx context.Context, id string) error {
	const msg_default = "hello from servo"

	msg := id
	if msg == "" {
		msg = msg_default
	}
	actualMessage, err := s.Echo(ctx, msg)
	if err != nil {
		return err
	}

	expectedMessage := "ECH0ING: " + msg
	if actualMessage != expectedMessage {
		return errors.Errorf("echo verification request returned %q; expected %q", actualMessage, expectedMessage)
	}

	return nil
}

// Restore servo back to the standard configuration.
func (s *Servo) Restore(ctx context.Context) error {
	var firstError error
	if s.initialPDRole != "" && s.initialPDRole != PDRoleNA {
		testing.ContextLogf(ctx, "Restoring %q to %q", PDRole, s.initialPDRole)
		if err := s.SetPDRole(ctx, s.initialPDRole); err != nil && firstError == nil {
			firstError = errors.Wrapf(err, "restoring servo control %q to %q", PDRole, s.initialPDRole)
		}
	}
	for v := range s.removedWatchdogs {
		testing.ContextLogf(ctx, "Restoring servo watchdog %q", v)
		if err := s.WatchdogAdd(ctx, v); err != nil && firstError == nil {
			firstError = errors.Wrapf(err, "restoring watchdog %q", v)
		}
	}
	return firstError
}

// Close performs Servo cleanup.
func (s *Servo) Close(ctx context.Context) error {
	return s.Restore(ctx)
}
