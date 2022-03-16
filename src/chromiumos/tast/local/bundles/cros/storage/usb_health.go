// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package storage

import (
	"bytes"
	"context"
	"regexp"
	"strings"

	"chromiumos/tast/common/storage"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/bundles/cros/storage/util"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         UsbHealth,
		Desc:         "Performs a SMART health check of usb drive",
		Contacts:     []string{"asavery@google.com"},
		SoftwareDeps: []string{"storage_wearout_detect"},
	})
}

const (
	// Healthy means that the device does not indicate failure or limited remaining life time.
	healthy storage.LifeStatus = iota
	// Failing indicates the storage device failed or will soon.
	failing
)

// runSmart runs the smartctl command to get health status
func runSmart(ctx context.Context, device string) ([]byte, error) {
	command := "smartctl -H " + device
	out, err := testexec.CommandContext(ctx, "sh", "-c", command).Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// parseUsbHealth parses the output of smartctl
func parseUsbHealth(ctx context.Context, outLines []string) storage.LifeStatus {
	usbPassed := regexp.MustCompile(`\s*SMART overall-health self-asssesment test result: PASSED.*`)

	for _, line := range outLines {
		testing.ContextLog(ctx, "output: ", line)
		match := usbPassed.FindStringSubmatch(line)
		if match != nil {
			testing.ContextLog(ctx, "Found match")
			return healthy
		}
	}

	return failing
}

// UsbHealth runs a SMART health test on a usb drive
func UsbHealth(ctx context.Context, s *testing.State) {
	usbDevice, err := util.RemovableDevice(ctx, false)
	if err != nil {
		s.Fatal("Failed to get removable device: ", err)
	}

	var out []byte
	out, err = runSmart(ctx, usbDevice)
	if err != nil {
		s.Fatal("Failed to run smartctl -H: ", err)
	}
	out = bytes.TrimSpace(out)
	lines := strings.Split(string(out), "\n")

	status := parseUsbHealth(ctx, lines)
	if status == failing {
		s.Fatal("usb device failed smartctl health check")
	}
}
