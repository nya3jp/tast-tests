// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"net"
	"os"
	"strings"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ServoNetwork,
		Desc:     "Verifies that the servo is connected",
		Contacts: []string{"jbettis@google.com"},
		Attr:     []string{"group:firmware", "firmware_unstable"},
		VarDeps:  []string{"servo"},
	})
}

// ServoNetwork attempts to verify that a machine in the lab can connect to servod without using ssh port forwarding.
// Delete this test after this is verified.
func ServoNetwork(ctx context.Context, s *testing.State) {
	// Check if test is running in the lab.
	// Lab machines might have hostnames starting with cautotest, or might end with cros.corp.google.com
	// Satlab machines start with satlab-
	// Moblab has the MOBLAB env var set. But we probably don't care about this case, because the servo string will be "${CONTAINER_NAME}:9999:docker:"
	hostname, err := os.Hostname()
	if err != nil {
		s.Error("Failed to get hostname: ", err)
	}
	s.Log("Hostname: ", hostname)
	if !strings.HasSuffix(hostname, ".cros.corp.google.com") {
		hostname += ".cros.corp.google.com"
	}

	if _, err := net.ResolveIPAddr("ip", hostname); err != nil {
		s.Log("Host is not in cros subnet: ", err)
	}
	serverEnv := os.Getenv("SERVER")
	s.Log("SERVER: ", serverEnv)
	moblabEnv := os.Getenv("MOBLAB")
	s.Log("MOBLAB: ", moblabEnv)

	doTest := func(ctx context.Context, servoHostPort string) {
		servoPxy, err := servo.NewProxy(ctx, servoHostPort, s.DUT().KeyFile(), s.DUT().KeyDir())
		if err != nil {
			s.Fatal("Failed to connect to servo: ", err)
		}
		defer servoPxy.Close(ctx)

		_, err = servoPxy.Servo().Echo(ctx, "any message")
		if err != nil {
			s.Fatal("Failed to ping servo: ", err)
		}

		err = servoPxy.Reconnect(ctx)
		if err != nil {
			s.Fatal("Failed to reconnect servo: ", err)
		}

		_, err = servoPxy.Servo().Echo(ctx, "any message")
		if err != nil {
			s.Fatal("Failed to ping servo: ", err)
		}

	}

	s.Log("Connecting to servo normally")
	servoHostPort, _ := s.Var("servo")
	doTest(ctx, servoHostPort)

	s.Log("Connecting to servo w/o ssh")
	servoHostPort += ":nossh"
	doTest(ctx, servoHostPort)
}
