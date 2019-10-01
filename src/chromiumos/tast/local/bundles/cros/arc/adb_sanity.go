// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"io/ioutil"
	"os"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ADBSanity,
		Desc:         "Verifies adb communication works as intended",
		Contacts:     []string{"hidehiko@chromium.org", "tast-owners@google.com"},
		SoftwareDeps: []string{"chrome", "android"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          arc.Booted(),
	})
}

func ADBSanity(ctx context.Context, s *testing.State) {
	a := s.PreValue().(arc.PreData).ARC
	testADBCommandStatus(ctx, s, a)
	testADBCommandLF(ctx, s, a)
}

// testADBCommandStatus exercises that arc.ARC.Command returns status code of the execution
// transparently.
func testADBCommandStatus(ctx context.Context, s *testing.State, a *arc.ARC) {
	if err := a.Command(ctx, "true").Run(); err != nil {
		s.Error("Unexpected error of true: ", err)
	}
	if err := a.Command(ctx, "false").Run(); err == nil {
		s.Error("Unexpected success of false: ", err)
	}
}

// testADBCommandLF exercises that arc.ARC.Command's output does NOT convert between LF and CRLF
// automatically. This ensures that the reading a file by "cat" works.
func testADBCommandLF(ctx context.Context, s *testing.State, a *arc.ARC) {
	const content = "abc\ndef\r\n"

	// Create a file containing the content.
	tmpfile, err := func() (string, error) {
		f, err := ioutil.TempFile("", "data")
		if err != nil {
			return "", err
		}
		success := false
		defer func() {
			if err := f.Close(); err != nil {
				s.Error("Failed to close a file: ", err)
			}
			if !success {
				os.Remove(f.Name())
			}
		}()
		if _, err := f.WriteString(content); err != nil {
			return "", err
		}
		success = true
		return f.Name(), nil
	}()
	if err != nil {
		s.Error("Failed to create a file: ", err)
		return
	}
	defer os.Remove(tmpfile)

	path, err := a.PushFileToTmpDir(ctx, tmpfile)
	if err != nil {
		s.Error("Failed to push a file: ", err)
		return
	}
	defer a.Command(ctx, "rm", path).Run(testexec.DumpLogOnError)

	// Read the file by "cat" command to make sure LF->CRLF conversion does NOT happen.
	if out, err := a.Command(ctx, "cat", path).Output(testexec.DumpLogOnError); err != nil {
		s.Error("Failed to read the file in Android: ", err)
		return
	} else if string(out) != content {
		s.Errorf("Unexpected read data: got %q; want %q", string(out), content)
		return
	}
}
