// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"bufio"
	"context"
	"io"
	"regexp"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         StartCrosvm,
		Desc:         "Checks that crosvm starts and runs commands",
		Contacts:     []string{"jkardatzke@chromium.org", "cros-containers-dev@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"vm_host"},
	})
}

func StartCrosvm(ctx context.Context, s *testing.State) {
	kernelArgs := []string{"-p", "init=/bin/bash"}
	cvm, err := vm.NewCrosvm(ctx, "", kernelArgs)
	if err != nil {
		s.Fatal("Failed to start crosvm: ", err)
	}
	defer cvm.Close(ctx)

	// Start a goroutine that reads bytes from crosvm and writes them to a channel.
	// We can't do this with lines because then we will miss the initial prompt
	// that comes up that doesn't have a line terminator.
	ch := make(chan byte)
	go func() {
		defer close(ch)
		r := bufio.NewReader(cvm.Stdout())
		for {
			b, err := r.ReadByte()
			if err == io.EOF {
				break
			} else if err != nil {
				s.Fatal("Failed reading from VM stdout: ", err)
			}
			ch <- b
		}
	}()

	// waitForOutput waits until a line matched by re has been written to ch,
	// crosvm's stdout is closed, or the deadline is reached. It returns the full
	// line that was matched.
	waitForOutput := func(re *regexp.Regexp) (string, error) {
		var line strings.Builder
		for {
			select {
			case c, more := <-ch:
				if !more {
					return "", errors.New("eof")
				}
				if c == '\n' {
					line.Reset()
				} else {
					line.WriteByte(c)
				}
				if re.MatchString(line.String()) {
					return line.String(), nil
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
