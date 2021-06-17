// Copyright 2022 The Chromium OS Authors. All rights reserved.
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

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/session"
	"chromiumos/tast/testing"
)

const (
	// numFiles and basePhrase should match cryptohome.SequentialConsistency.filewriter.cc
	numFiles   = 9
	basePhrase = "This is file #%d"

	subprocessPath = "/usr/local/libexec/tast/helpers/local/cros/cryptohome.SequentialConsistency.filewriter"
)

type sequentialConsistencyParams struct {
	subdirectory bool
	user         string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         SequentialConsistency,
		Desc:         "Checks that different processes don't have inconsistent views of cryptohome",
		Contacts:     []string{"iby@chromium.org", "chromeos-security@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
		// DO NOT SUBMIT: Also add "informational" after dry-run pass.
		Pre: chrome.LoggedIn(),
		Params: []testing.Param{{
			Name: "normal",
			Val: sequentialConsistencyParams{
				subdirectory: false,
				user:         "root",
			},
		}, {
			Name: "normal_chronos",
			Val: sequentialConsistencyParams{
				subdirectory: false,
				user:         "chronos",
			},
		}, {
			Name: "subdirectory",
			Val: sequentialConsistencyParams{
				subdirectory: true,
				user:         "root",
			},
		}, {
			Name: "subdirectory_chronos",
			Val: sequentialConsistencyParams{
				subdirectory: true,
				user:         "chronos",
			},
		}}})
}

// makeFilePath here should match makeFilePath in cryptohome.SequentialConsistency.filewriter.cc.
func makeFilePath(path string, index int) string {
	return filepath.Join(path, fmt.Sprint("SequentialConsistencyTest.", index, ".txt"))
}

func SequentialConsistency(ctx context.Context, s *testing.State) {
	// Have a different process write files into cryptohome. Ensure that this
	// process sees the writes occur in the same order that the process writes
	// them.
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

	params := s.Param().(sequentialConsistencyParams)
	if params.subdirectory {
		path = filepath.Join(path, "SequentialConsistency")
		// Clean up leftovers from any previous tests.
		if err := os.RemoveAll(path); err != nil && !os.IsNotExist(err) {
			s.Fatal("Cannot remove ", path, ": ", err)
		}
		defer os.RemoveAll(path)
	} else {
		// Clean up leftovers from any previous tests
		for i := 0; i < numFiles; i++ {
			if err := os.Remove(makeFilePath(path, i)); err != nil && !os.IsNotExist(err) {
				s.Fatal("Cannot remove ", makeFilePath(path, i), ": ", err)
			}
			defer os.Remove(makeFilePath(path, i))
		}
	}

	// Start the subprocess.
	var command []string
	if params.user != "root" {
		command = []string{"sudo", "--non-interactive", "--user=" + params.user, "--"}
	}
	command = append(command, subprocessPath)
	if params.subdirectory {
		command = append(command, "--create_dir")
	}
	command = append(command, "--path="+path)
	testing.ContextLog(ctx, "Starting ", command)
	cmd := testexec.CommandContext(ctx, command[0], command[1:]...)
	if err := cmd.Start(); err != nil {
		s.Fatalf("Failed to start %v: %v", command, err)
	}

	// Leave some time for cleanup.
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// We expect the following things to happen in this order:
	// File #1 is created. File #1 has a non-decreasing # of bytes. File #1 has
	// the complete string in it. File #2 is created. File #2 has a non-decreasing
	// # of bytes. File #2 has the complete string in it.  Etc.
	// Error out if we go backwards or if things happen out of order.
	var fileCreated [numFiles]bool
	var lastFileSize [numFiles]int
	var fileCompleted [numFiles]bool
	var completionTime time.Time

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// Run through the files backwards.
		//
		// If we run through the files in ascending order, we can't say anything
		// about whether or not a file should exist. For instance, if file 1 exists,
		// it's fine for file 2 to exist but it's also fine for file 2 to not exist
		// (the subprogram hasn't gotten to it yet). If file 1 doesn't exist, it's
		// normal for file 2 to not exist, but it's also possible that file 2 will
		// exist because the subprogram wrote both file 1 and file 2 in the delay
		// between checking file 1 and file 2.
		//
		// However, if we run through the files backwards, we can say that if file 3
		// exists, file 2 should definitely exist and be complete.
		for i := numFiles - 1; i >= 0; i-- {
			fileName := makeFilePath(path, i)
			if _, err := os.Stat(fileName); err == nil {
				// File exists.
				fileCreated[i] = true
				contents, err := ioutil.ReadFile(fileName)
				if err != nil {
					return testing.PollBreak(errors.Wrapf(err, "could not read %s", fileName))
				}

				expectedPhrase := fmt.Sprintf(basePhrase, i)
				if !bytes.HasPrefix([]byte(expectedPhrase), contents) {
					return testing.PollBreak(errors.Errorf("file %v did not contain expected phrase. Instead contained %q", fileName, contents))
				}

				if lastFileSize[i] > len(contents) {
					return testing.PollBreak(errors.Errorf("file %v shrunk. Previously contained %d bytes, now contains %d bytes", fileName, lastFileSize[i], len(contents)))
				}
				lastFileSize[i] = len(contents)
				if lastFileSize[i] == len(expectedPhrase) {
					fileCompleted[i] = true
				}
			} else if os.IsNotExist(err) {
				if fileCreated[i] {
					return testing.PollBreak(errors.Errorf("file %v disappeared after creation", fileName))
				}
			} else {
				return testing.PollBreak(errors.Wrapf(err, "cannot stat %v", fileName))
			}
		}

		for i := 1; i < numFiles; i++ {
			if fileCreated[i] && !fileCompleted[i-1] {
				return testing.PollBreak(errors.Errorf("file #%d created before file #%d was completed", i, i-1))
			}
		}

		if fileCompleted[numFiles-1] {
			// All the files are complete. Wait a short while to make sure nothing
			// flickers after completion.
			if completionTime.IsZero() {
				completionTime = time.Now()
			} else if time.Now().After(completionTime.Add(10 * time.Second)) {
				// End loop
				return nil
			}
		}
		// Don't stop until the completionTime + 10 seconds is reached.
		waitingFor := ""
		for i := 0; i < numFiles && waitingFor == ""; i++ {
			if !fileCreated[i] {
				if i == 0 || fileCompleted[i-1] {
					waitingFor = fmt.Sprintf("file #%d to be created", i)
				} else {
					waitingFor = fmt.Sprintf("file #%d to be finished", i-1)
				}
			}
		}
		if waitingFor == "" {
			if fileCompleted[numFiles-1] {
				waitingFor = "timeout after final file completed"
			} else {
				waitingFor = fmt.Sprintf("file #%d to be finished", numFiles-1)
			}
		}
		return errors.Errorf("so far so good but not done yet. Waiting for %s", waitingFor)
	}, nil); err != nil {
		s.Fatal("Loop failed: ", err)
	}
}
