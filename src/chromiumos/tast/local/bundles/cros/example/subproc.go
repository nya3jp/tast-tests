// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     Subproc,
		Desc:     "Demonstrates how to define and run subprocesses",
		Attr:     []string{"informational"},
		Subprocs: []testing.SubprocFunc{Subproc_CheckUID},
	})
}

func Subproc(ctx context.Context, s *testing.State) {
	// runCmd executes the supplied command line and checks that
	// it writes expUID to stdout.
	runCmd := func(cl []string, expUID int) {
		cmd := testexec.CommandContext(ctx, cl[0], cl[1:]...)
		s.Log("Running command: ", shutil.EscapeSlice(cmd.Args))
		out, err := cmd.Output()
		if err != nil {
			s.Errorf("Failed to execute command %s: %v", cmd.Args, err)
			cmd.DumpLog(ctx)
			return
		}

		actual := strings.TrimSpace(string(out))
		exp := strconv.Itoa(expUID)
		s.Logf("Got output %q", actual)
		if actual != exp {
			s.Errorf("Subprocess printed %q; wanted %q", actual, exp)
		}
	}

	// First, just run the subprocess using our own UID.
	uid := os.Getuid()
	runCmd(testing.SubprocCommand(Subproc_CheckUID, fmt.Sprintf("-uid=%d", uid)), uid)

	// Now run the subprocess as a different user.
	const username = "chronos"
	otherUID, err := sysutil.GetUID(username)
	if err != nil {
		s.Fatal("Failed to look up user: ", err)
	}
	subCmd := testing.SubprocCommand(Subproc_CheckUID, fmt.Sprintf("-uid=%d", otherUID))
	runCmd([]string{"su", username, "-c", shutil.EscapeSlice(subCmd)}, int(otherUID))
}

// Subproc_CheckUID reads a 'uid' flag from args, checks that it is running as the
// expected UID, and prints the current UID to stdout.
func Subproc_CheckUID(args []string) {
	flags := flag.NewFlagSet("", flag.ExitOnError)
	expUID := flags.Int("uid", -1, "expected UID")
	if err := flags.Parse(args); err != nil {
		log.Fatal("Failed to parse flags: ", err)
	}
	uid := os.Getuid()
	if uid != *expUID {
		log.Fatalf("Running as UID %d; want %d", uid, *expUID)
	}
	fmt.Println(uid)
}
