// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

import (
	"context"
	"io/ioutil"
	"os"
	"regexp"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MonitorUsbEvent,
		Desc:         "Monitors the USB event detected properly or not",
		Contacts:     []string{"cros-tdm-tpe-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "crosHealthdRunning",
	})
}

func MonitorUsbEvent(ctx context.Context, s *testing.State) {
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	outFile, err := ioutil.TempFile("", "usb_logs")
	if err != nil {
		s.Fatal("Failed to create temp file: ", err)
	}
	defer os.Remove(outFile.Name())

	// Run monitor command in background.
	monitorCmd := testexec.CommandContext(ctx, "cros-health-tool", "event", "--category=usb", "--length_seconds=10")
	monitorCmd.Stdout = outFile
	monitorCmd.Stderr = outFile
	if err := monitorCmd.Start(); err != nil {
		outFile.Close()
		s.Fatal("Failed to run healthd monitor command: ", err)
	}

	// Trigger USB event.
	if err := testexec.CommandContext(ctx, "udevadm", "trigger", "-s", "usb", "-c", "add").Run(); err != nil {
		s.Fatal("Failed to trigger usb add event: ", err)
	}

	monitorCmd.Wait()
	outFile.Close()

	deviceAddedPattern := regexp.MustCompile(`"event": "Add"`)
	if output, err := ioutil.ReadFile(outFile.Name()); err != nil {
		s.Fatal("Failed to read output data: ", err)
	} else if !deviceAddedPattern.MatchString(string(output)) {
		s.Fatal("Failed to detect USB Event, event output: ", string(output))
	}
}
