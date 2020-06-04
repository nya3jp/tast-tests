// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"bytes"
	"context"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const (
	vmPstoreDumpPath   = "/usr/bin/vm_pstore_dump"
	ramoopsConsoleSize = 0x40000                 // ramoops.console_size, the kernel parameter
	consoleBufferSize  = ramoopsConsoleSize - 12 // ramoops.console_size - persistent_ram_buffer.sig (4 byte) - persistent_ram_buffer.start (4 byte) - persistent_ram_buffer.size (4 byte)
	pollingInterval    = 1.0 * time.Second
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VMPstoreDump,
		Desc:         "Test of vm_pstore_dump command",
		Contacts:     []string{"kimiyuki@google.com", "arcvm-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "android_vm"},
		Pre:          arc.VMBooted(),
		Params: []testing.Param{{
			Name: "no_options",
			Val:  false,
		}, {
			Name: "use_follow",
			Val:  true,
		}},
	})
}

// waitPolling waits the `vm_pstore_dump --follow` command to read the update of .pstore file.
func waitPolling(ctx context.Context) {
	testing.Sleep(ctx, time.Duration(math.Floor(1.1*float64(pollingInterval))))
}

// sendToKmsg sends msg to the kernel's console output.
func sendToKmsg(ctx context.Context, msg string) error {
	return testexec.CommandContext(ctx,
		"android-sh", "-c", "echo "+msg+"> /dev/kmsg").Run()
}

// fillKmsg fill the ring buffer for the kernel's console output in the .pstore file.
func fillKmsg(ctx context.Context) error {
	const longString = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	stringCount := consoleBufferSize/len(longString) + 100
	shellScript1 := "for i in `seq " + strconv.Itoa(stringCount/2) + "` ; do echo 1 $i " + longString + "  > /dev/kmsg ; done"
	shellScript2 := "for i in `seq " + strconv.Itoa(stringCount/2) + "` ; do echo 2 $i " + longString + "  > /dev/kmsg ; done"

	if err := testexec.CommandContext(ctx, "android-sh", "-c", shellScript1).Run(); err != nil {
		return nil
	}
	waitPolling(ctx)
	return testexec.CommandContext(ctx, "android-sh", "-c", shellScript2).Run()
}

func vmPstoreDumpAfterBoot(ctx context.Context, s *testing.State) {
	// write a marker string to console output
	const markerString = "mv_pstore_dump-test-marker"
	if err := sendToKmsg(ctx, markerString); err != nil {
		s.Fatal("Failed to write to /dev/kmsg: ", err)
	}

	// run the vm_pstore_dump command
	buf, err := testexec.CommandContext(ctx, vmPstoreDumpPath).Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to get the output of vm_pstore_dump command: ", err)
	}
	if !utf8.Valid(buf) {
		s.Fatal("The output is not a valid UTF-8 stiring")
	}
	out := string(buf)

	// check the output of the command
	if len(buf) > consoleBufferSize {
		s.Errorf("The output is too long. It must be less than or equal to the buffer size (%d): %d", consoleBufferSize, len(buf))
	}
	matched, err := regexp.MatchString(`^\[ *\d+\.\d+\] Linux version `, out)
	if err != nil || !matched {
		s.Error("Kernel's console output after booting should start with a string like \"[    0.000000] Linux version ...\" but it's not found in the result")
	}
	if !strings.Contains(out, markerString) {
		s.Error("The marker string we wrote to kernel's console output is not found in the output")
	}
}

func vmPstoreDumpAfterLogRotation(ctx context.Context, s *testing.State) {
	// write strings to console output
	const (
		headerMarkerString = "mv_pstore_dump-test-marker-first"
		footerMarkerString = "mv_pstore_dump-test-marker-last"
	)
	if err := sendToKmsg(ctx, headerMarkerString); err != nil {
		s.Fatal("Failed to write to /dev/kmsg: ", err)
	}
	if err := fillKmsg(ctx); err != nil {
		s.Fatal("Failed to write to /dev/kmsg: ", err)
	}
	if err := sendToKmsg(ctx, footerMarkerString); err != nil {
		s.Fatal("Failed to write to /dev/kmsg: ", err)
	}

	// run the vm_pstore_dump command
	buf, err := testexec.CommandContext(ctx, vmPstoreDumpPath).Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to get the output of vm_pstore_dump command: ", err)
	}
	if !utf8.Valid(buf) {
		s.Fatal("The output is not a valid UTF-8 string")
	}
	out := string(buf)

	// check the output of the command
	if len(buf) < int(math.Floor(consoleBufferSize*0.99)) {
		s.Errorf("The output is too short. It's expected about the buffer size %d: %d", consoleBufferSize, len(buf))
	}
	if len(buf) > consoleBufferSize {
		s.Errorf("The output is too long. It must be less than or equal to the buffer size (%d): %d", consoleBufferSize, len(buf))
	}
	if strings.Contains(out, headerMarkerString) {
		s.Errorf("The marker string (%v) we wrote once to kernel's console output and cleared is found in the output", headerMarkerString)
	}
	if !strings.Contains(out, footerMarkerString) {
		s.Errorf("The marker string (%v) we wrote to kernel's console output is not found in the output", footerMarkerString)
	}
}

func vmPstoreDumpWithFollow(ctx context.Context, s *testing.State) {
	// write a marker string to console output
	const firstMarkerString = "mv_pstore_dump-test-marker-first"
	if err := sendToKmsg(ctx, firstMarkerString); err != nil {
		s.Fatal("Failed to write to /dev/kmsg: ", err)
	}

	// start the vm_pstore_dump command
	cmd := testexec.CommandContext(ctx, vmPstoreDumpPath, "--follow")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Start(); err != nil {
		s.Fatal("Failed to start vm_pstore_dump command: ", err)
	}
	defer cmd.Kill()

	// write another strings to console output
	const (
		secondMarkerString = "mv_pstore_dump-test-marker-second"
		lastMarkerString   = "mv_pstore_dump-test-marker-last"
	)
	if err := sendToKmsg(ctx, secondMarkerString); err != nil {
		s.Fatal("Failed to write to /dev/kmsg: ", err)
	}
	if err := fillKmsg(ctx); err != nil {
		s.Fatal("Failed to write to /dev/kmsg: ", err)
	}
	if err := sendToKmsg(ctx, lastMarkerString); err != nil {
		s.Fatal("Failed to write to /dev/kmsg: ", err)
	}

	// stop the vm_pstore_dump command
	waitPolling(ctx)
	if err := cmd.Kill(); err != nil {
		s.Error("Failed to kill vm_pstore_dump commnd: ", err)
	}
	buf := stdout.Bytes()
	if !utf8.Valid(buf) {
		s.Fatal("The output is not a valid UTF-8 string")
	}
	out := string(buf)

	// check the output of the command
	if len(buf) <= consoleBufferSize {
		s.Errorf("The output is too short. It must be at least the buffer size (%d): %d", consoleBufferSize, len(buf))
	}
	if !strings.Contains(out, firstMarkerString) {
		s.Errorf("The marker string (%v) we wrote to kernel's console output is not found in the output", firstMarkerString)
	}
	if !strings.Contains(out, secondMarkerString) {
		s.Errorf("The marker string (%v) we wrote to kernel's console output is not found in the output", secondMarkerString)
	}
	if !strings.Contains(out, lastMarkerString) {
		s.Errorf("The marker string (%v) we wrote to kernel's console output is not found in the output", lastMarkerString)
	}
}

func VMPstoreDump(ctx context.Context, s *testing.State) {
	useFollow := s.Param().(bool)
	if useFollow {
		vmPstoreDumpWithFollow(ctx, s)
	} else {
		vmPstoreDumpAfterBoot(ctx, s)
		vmPstoreDumpAfterLogRotation(ctx, s)
	}
}
