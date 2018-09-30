// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"bufio"
	"context"
	"errors"
	"io"
	"regexp"

	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"

	"github.com/godbus/dbus"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         StartCrosvm,
		Desc:         "Checks that crosvm starts and runs commands",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"vm_host"},
	})
}

func StartCrosvm(ctx context.Context, s *testing.State) {
	bus, err := dbus.SystemBus()
	if err != nil {
		s.Fatal("Failed to connect to D-Bus: ", err)
	}
	const svc = "org.chromium.ComponentUpdaterService"
	if err := dbusutil.WaitForService(ctx, bus, svc); err != nil {
		s.Fatalf("Failed waiting for %v: %v", svc, err)
	}

	kernelArgs := []string{"-p", "init=/bin/bash"}
	cvm, err := vm.NewCrosvm(ctx, "", kernelArgs)
	if err != nil {
		s.Fatal("Failed to start crosvm: ", err)
	}
	defer cvm.Close(ctx)

	// Start a goroutine that reads lines from crosvm and writes them to a channel.
	ch := make(chan string)
	go func() {
		sc := bufio.NewScanner(cvm.Stdout())
		for sc.Scan() {
			ch <- sc.Text()
		}
		close(ch)
	}()

	// waitForOutput waits until a line matched by re is written to ch, crosvm's stdout is closed, or the deadline is reached.
	// It returns the full line that was matched.
	waitForOutput := func(re *regexp.Regexp) (string, error) {
		for {
			select {
			case line, more := <-ch:
				if !more {
					return "", errors.New("eof")
				}
				if re.MatchString(line) {
					return line, nil
				}
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}
	}

	testing.ContextLog(ctx, "Waiting for VM to boot")
	line, err := waitForOutput(regexp.MustCompile("localhost\\b.*#"))
	if err != nil {
		s.Fatal("Didn't get VM prompt: ", err)
	}
	s.Logf("Saw prompt in line %q", line)

	const cmd = "/bin/ls -1 /"
	s.Logf("Running %q", cmd)
	if _, err = io.WriteString(cvm.Stdin(), cmd+"\n"); err != nil {
		s.Fatalf("Failed to write %q command: %v", cmd, err)
	}
	if line, err = waitForOutput(regexp.MustCompile("^sbin$")); err != nil {
		s.Errorf("Didn't get expected %q output: %v", cmd, err)
	} else {
		s.Logf("Saw line %q", line)
	}
}
