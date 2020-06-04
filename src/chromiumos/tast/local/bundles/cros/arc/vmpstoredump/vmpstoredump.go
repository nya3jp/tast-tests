// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package vmpstoredump provides constants and utilities to test vm_pstore_dump command.
package vmpstoredump

import (
	"context"
	"math"
	"strconv"
	"time"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const (
	// VMPstoreDumpPath is the path to vm_pstore_dump command.
	VMPstoreDumpPath = "/usr/bin/vm_pstore_dump"
	// ramoops.console_size, the kernel parameter
	ramoopsConsoleSize = 0x40000
	// ConsoleBufferSize is the expected size of the ring buffer for the console output. The size is ramoops.console_size - persistent_ram_buffer.sig (4 byte) - persistent_ram_buffer.start (4 byte) - persistent_ram_buffer.size (4 byte)
	ConsoleBufferSize = ramoopsConsoleSize - 12
	pollingInterval   = 1.0 * time.Second
)

// WaitPolling waits the `vm_pstore_dump --follow` command to read the update of .pstore file.
func WaitPolling(ctx context.Context) {
	testing.Sleep(ctx, time.Duration(math.Floor(1.1*float64(pollingInterval))))
}

// SendToKmsg sends msg to the kernel's console output.
func SendToKmsg(ctx context.Context, msg string) error {
	return testexec.CommandContext(ctx,
		"android-sh", "-c", "echo "+msg+"> /dev/kmsg").Run()
}

// FillKmsg fill the ring buffer for the kernel's console output in the .pstore file.
func FillKmsg(ctx context.Context) error {
	const longString = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	stringCount := ConsoleBufferSize/len(longString) + 100

	// Scripts to write enough amount of strings to fill the ring buffer. The prefixes "1 $i" and "2 $i" is only to help debugging.
	shellScript1 := "for i in `seq " + strconv.Itoa(stringCount/2) + "` ; do echo 1 $i " + longString + "  > /dev/kmsg ; done"
	shellScript2 := "for i in `seq " + strconv.Itoa(stringCount/2) + "` ; do echo 2 $i " + longString + "  > /dev/kmsg ; done"

	if err := testexec.CommandContext(ctx, "android-sh", "-c", shellScript1).Run(); err != nil {
		return nil
	}
	WaitPolling(ctx) // to ensure that all strings are treated by vm_pstore_dump command
	return testexec.CommandContext(ctx, "android-sh", "-c", shellScript2).Run()
}
