// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cryptohome

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/session"
	"chromiumos/tast/testing"
)

type fileStabilityParams struct {
	subdirectory bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         FileStability,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that the cryptohome is stable after login",
		Contacts:     []string{"iby@chromium.org", "chromeos-security@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          chrome.LoggedIn(),
		Params: []testing.Param{{
			Name: "normal",
			Val: fileStabilityParams{
				subdirectory: false,
			},
		}, {
			Name: "subdirectory",
			Val: fileStabilityParams{
				subdirectory: true,
			},
		}}})
}

// keepGoingError is a fake error type used to keep testing.Poll from returning
// before the timeout. It indicates the lack of any error.
type keepGoingError struct{}

func (e *keepGoingError) Error() string {
	return "fake error to keep testing.Poll going"
}

func FileStability(ctx context.Context, s *testing.State) {
	// Create several random files inside cryptohome. Assert that they remain
	// 'visible' and readable after being written.
	fileContents := [][]byte{
		[]byte("Hello"),
		[]byte("This is the second file"),
		[]byte("This is a long file "), // More text is added below.
		[]byte("\000control characters\001"),
	}

	// Make one file reasonably long. We want a pattern not just a set of zeroes
	// to detect rearrangements or unrecorded gaps in the file.
	b := byte(0)
	for len(fileContents[2]) < 10000 {
		fileContents[2] = append(fileContents[2], b)
		b++
	}

	sm, err := session.NewSessionManager(ctx)
	if err != nil {
		s.Fatal("Could not connect to session_manager: ", err)
	}

	sessions, err := sm.RetrieveActiveSessions(ctx)
	if err != nil {
		s.Fatal("Could not retrieve active sessions: ", err)
	}

	// chrome.LoggedIn should promise we only have 1 active session.
	if len(sessions) != 1 {
		s.Fatal("Unexpected # of sessions. Expected 1, got ", sessions)
	}

	var path string
	for _, hashedName := range sessions {
		path = filepath.Join("/home/user", hashedName)
	}

	params := s.Param().(fileStabilityParams)
	if params.subdirectory {
		path = filepath.Join(path, "FileStabilityDirectory")
		os.RemoveAll(path) // Clean up leftovers from any previous tests.
		if err := os.Mkdir(path, 0755); err != nil {
			s.Fatal("Error creating directory ", path, ": ", err)
		}
		defer os.RemoveAll(path)
	}

	var fileNames []string
	for i := 0; i < len(fileContents); i++ {
		fileName := filepath.Join(path, fmt.Sprint("FileStabilityTest.", i, ".txt"))
		fileNames = append(fileNames, fileName)
		if err := ioutil.WriteFile(fileName, []byte(fileContents[i]), 0644); err != nil {
			s.Fatal("Could not write ", fileName, ": ", err)
		}
		defer os.Remove(fileName)

		// Immediately attempt a read.
		if contents, err := ioutil.ReadFile(fileName); err != nil {
			s.Fatal("Could not immediately re-read ", fileName, ": ", err)
		} else if bytes.Compare(contents, []byte(fileContents[i])) != 0 {
			s.Fatalf("Immediate re-read of %s did not get expected result: Expect %q, got %q", fileName, fileContents[i], contents)
		}
	}

	keepGoingErr := &keepGoingError{}
	testing.ContextLog(ctx, "Polling for file stability")

	// Confirm files continue to exist & be readable for 20 seconds.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		for i := 0; i < len(fileNames); i++ {
			fileName := fileNames[i]
			if contents, err := ioutil.ReadFile(fileName); err != nil {
				return testing.PollBreak(errors.Wrapf(err, "could not later re-read %s", fileName))
			} else if bytes.Compare(contents, []byte(fileContents[i])) != 0 {
				return testing.PollBreak(errors.Errorf("later re-read of %s did not get expected result: Expect %q, got %q", fileName, fileContents[i], contents))
			}
		}
		// Don't stop until the full timeout is reached.
		return keepGoingErr
	}, &testing.PollOptions{Timeout: 20 * time.Second}); !errors.Is(err, keepGoingErr) {
		s.Fatal("Loop failed: ", err)
	}
}
