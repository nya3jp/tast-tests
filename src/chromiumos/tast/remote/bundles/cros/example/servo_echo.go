// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"
	"fmt"

	"chromiumos/tast/host"
	"chromiumos/tast/remote/servo"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ServoEcho,
		Desc:     "Demonstrates running a test using Servo",
		Contacts: []string{"jeffcarp@chromium.org", "derat@chromium.org", "tast-users@chromium.org"},
		Attr:     []string{"disabled", "informational"},
	})
}

// ServoEcho demonstrates how you'd use Servo in a Tast test using the echo method.
func ServoEcho(ctx context.Context, s *testing.State) {
	// TODO(jeffcarp): Parameterize servod host and port.
	const msg = "hello from servo"

	// TODO(CL): Just hard-coding these for now =============
	servodBotAddress := "chromeos15-row14a-rack6-host5-servo.cros"
	remoteAddr := "localhost:9999"
	keyFile := "/home/jeffcarp/.ssh/testing_rsa"
	// ======================================================

	opts := host.SSHOptions{Hostname: servodBotAddress, Port: 22, User: "root",
		KeyFile: keyFile}
	ssh, err := host.NewSSH(ctx, &opts)
	if err != nil {
		s.Fatal("error setting up SSH for servod proxy: ", err)
	}

	proxy, err := servo.NewSSHServodProxy(ctx, ssh, remoteAddr)
	defer proxy.Close()
	if err != nil {
		s.Fatal("error creating new servod proxy: ", err)
	}
	s.Log("DEBUG: set up servod proxy: %v", proxy)

	localServodTunnelAddr := fmt.Sprintf("127.0.0.1:%d", proxy.LocalPort())

	svo, err := servo.NewServo(ctx, localServodTunnelAddr)
	if err != nil {
		s.Fatal("Servo init error: ", err)
	}

	actualMessage, err := svo.Echo(ctx, "hello from servo")
	if err != nil {
		s.Fatal("Got error: ", err)
	}
	const expectedMessage = "ECH0ING: " + msg
	if actualMessage != expectedMessage {
		s.Fatalf("Got message %q; expected %q", actualMessage, expectedMessage)
	}
}
