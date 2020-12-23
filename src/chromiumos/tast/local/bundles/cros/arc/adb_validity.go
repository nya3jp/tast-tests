// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ADBValidity,
		Desc:         "Verifies adb communication works as intended",
		Contacts:     []string{"hidehiko@chromium.org", "tast-owners@google.com"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBooted",
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func ADBValidity(ctx context.Context, s *testing.State) {
	a := s.FixtValue().(*arc.PreData).ARC
	testADBCommandStatus(ctx, s, a)

	var allbytes [256]byte
	for i := range allbytes {
		allbytes[i] = byte(i)
	}

	tmpdir, err := a.TempDir(ctx)
	if err != nil {
		s.Fatal("Failed to create tempdir: ", err)
	}
	defer func() {
		if err := a.RemoveAll(ctx, tmpdir); err != nil {
			s.Error("Failed to clean up tmpdir: ", err)
		}
	}()
	for i, content := range [][]byte{
		[]byte("abc\ndef\n"),     // content with \n
		[]byte("abc\rdef\r"),     // content with \r
		[]byte("abc\r\ndef\r\n"), // content with \r\n
		[]byte("abcdef"),         // content with no \r nor \n
		allbytes[:],              // containing all possible bytes.
	} {
		workdir := fmt.Sprintf("%s/%d", tmpdir, i)
		if err := a.Command(ctx, "mkdir", workdir).Run(testexec.DumpLogOnError); err != nil {
			s.Fatal("Failed to create a working dir: ", err)
		}
		testADBCommandStdin(ctx, s, a, workdir, content)
		testADBCommandStdout(ctx, s, a, workdir, content)
	}
}

// testADBCommandStatus checks arc.ARC.Command returns status code of the execution
// transparently.
func testADBCommandStatus(ctx context.Context, s *testing.State, a *arc.ARC) {
	if err := a.Command(ctx, "true").Run(); err != nil {
		s.Error("Unexpected error of true: ", err)
	}
	if err := a.Command(ctx, "false").Run(); err == nil {
		s.Error("Unexpected success of false: ", err)
	}
}

// testADBCommandStdin checks arc.ARC.Command's stdin does NOT convert
// between LF and CRLF automatically.
func testADBCommandStdin(ctx context.Context, s *testing.State, a *arc.ARC, tmpdir string, content []byte) {
	path := filepath.Join(tmpdir, "stdin.txt")

	// "tee" dumps the stdin to the given path (and stdout).
	cmd := a.Command(ctx, "tee", path)
	cmd.Stdin = bytes.NewBuffer(content)
	if err := cmd.Run(); err != nil {
		s.Error("Failed to execute the command: ", err)
		return
	}

	if out, err := a.ReadFile(ctx, path); err != nil {
		s.Error("Failed to read the file in Android: ", err)
	} else if !bytes.Equal(out, content) {
		s.Errorf("Unexpected data: got %q; want %q", out, content)
	}
}

// testADBCommandStdout checks arc.ARC.Command's output does NOT convert
// between LF and CRLF automatically.
func testADBCommandStdout(ctx context.Context, s *testing.State, a *arc.ARC, tmpdir string, content []byte) {
	path := filepath.Join(tmpdir, "stdout.txt")

	if err := a.WriteFile(ctx, path, content); err != nil {
		s.Error("Failed to create a file: ", err)
		return
	}

	// Read the file by "cat" command to make sure LF->CRLF conversion does NOT happen.
	if out, err := a.Command(ctx, "cat", path).Output(testexec.DumpLogOnError); err != nil {
		s.Error("Failed to read the file in Android: ", err)
	} else if !bytes.Equal(out, content) {
		s.Errorf("Unexpected read data: got %q; want %q", out, content)
	}
}
