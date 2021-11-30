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
	"chromiumos/tast/errors"
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
	var (
		deviceAdded           = regexp.MustCompile(`"event": "Add"`)
		outFile               = "/tmp/usb_logs.txt"
		usbMonitorCommand     = "sudo nohup cros-health-tool event --category=usb --length_seconds=600 > " + outFile + " 2>&1 &"
		pidCmd                = "ps -aux | grep -i nohup | awk -F' ' '{print $2}' | head -1"
		killUsbMonitorCommand = "kill -9 $(" + pidCmd + ")"
	)

	if _, err := os.Stat("/sys/bus/usb/devices/"); os.IsNotExist(err) {
		s.Log("No usb devices, skip the test")
		return
	}

	ctxForCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()
	defer func() {
		os.RemoveAll(outFile)
		if err := testexec.CommandContext(ctxForCleanUp, "sh", "-c", killUsbMonitorCommand).Run(); err != nil {
			s.Log("Failed to kill command usbMonitorCommand execution: ", err)
		}
	}()
	getUsbEventOutput := func() string {
		output, err := ioutil.ReadFile(outFile)
		if err != nil {
			s.Fatal("Failed to read data: ", err)
		}
		return string(output)
	}

	// Run monitor command in background.
	if err := testexec.CommandContext(ctx, "sh", "-c", usbMonitorCommand).Run(); err != nil {
		s.Fatal("Failed to run monitor event: ", err)
	}

	// Trigger USB event.
	if err := testexec.CommandContext(ctx, "udevadm", "trigger", "-s", "usb", "-c", "add").Run(); err != nil {
		s.Fatal("Failed to trigger usb add event: ", err)
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		output := getUsbEventOutput()
		if !deviceAdded.MatchString(output) {
			return errors.Errorf("failed to detect USB Event, event output: %v", output)
		}
		return nil
	}, &testing.PollOptions{Timeout: 15 * time.Second}); err != nil {
		s.Fatal("Failed to verify USB events: ", err)
	}
}
