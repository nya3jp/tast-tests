// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"net"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/bundles/cros/platform/screenlatency"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ScreenLatency,
		Desc:         "Tests latency between pressing a key and having it shown on a screen",
		Contacts:     []string{"mblsha@google.com"},
		Attr:         []string{},
		SoftwareDeps: []string{},
		Params:       []testing.Param{},
	})
}

// ScreenLatency uses a companion Android app to measure latency between a
// key press being simulated and a character appearing on the screen.
//
// TODO(mblsha): See the http://go/project-slate-handover for future direction info.
func ScreenLatency(ctx context.Context, s *testing.State) {
	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create keyboard device: ", err)
	}

	ln, _ := net.Listen("tcp", "127.0.0.1:")
	_, serverPort, _ := net.SplitHostPort(ln.Addr().String())
	testing.ContextLog(ctx, "Listening on address: ", ln.Addr())

	openAppCommand := testexec.CommandContext(ctx, "adb", "shell", "am", "start", "-n",
		"com.android.example.camera2.slowmo/com.example.android.camera2.slowmo.CameraActivity",
		"--es", "port "+serverPort)
	if err := openAppCommand.Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to start companion Android app using adb")
	}

	portForwardingCommand := testexec.CommandContext(ctx, "adb", "reverse", "tcp:"+serverPort, "tcp:"+serverPort)
	if err := portForwardingCommand.Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to initiate TCP connection with a companion Android app")
	}

	screenlatency.CommunicateWithCompanionApp(ctx, s, ln, keyboard)
}
