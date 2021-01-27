// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package printer

import (
	"bufio"
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const (
	adminTimeout     = 20 * time.Second
	serverTagTimeout = 20 * time.Second

	ippCmdLine = "./test/start-server.sh"

	serverAddressTag = "Using default listeners for "
	startingTag      = "Starting HTTPS session."
	encryptedTag     = "Connection now encrypted."
)

func init() {
	testing.AddTest(&testing.Test{
		Func: GnuTLS,
		Desc: "TODO",
		Contacts: []string{
			"mwojtas@google.com",
		},
		SoftwareDeps: []string{"cros_internal", "cups"},
		Attr:         []string{"group:mainline"}, // TODO group ok?
	})
}

func waitForTag(ctx context.Context, lines chan string, tag string) (line string, err error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, serverTagTimeout)
	defer cancel()

	for {
		select {
		case <-timeoutCtx.Done():
			return "", errors.New("Timeout waiting for tag " + tag)
		case line := <-lines:
			if strings.Contains(line, tag) {
				return line, nil
			}
		}
	}
}

func waitForServerAddress(ctx context.Context, lines chan string) (string, error) {
	line, err := waitForTag(ctx, lines, serverAddressTag)

	if err != nil {
		return "", errors.Wrap(err, "failed waiting for server address")
	}

	idx := strings.Index(line, serverAddressTag)
	return line[idx+len(serverAddressTag) : len(line)-1], nil
}

func startServer(ctx context.Context, s *testing.State) (outputLines chan string, address string) {
	cmdIPP := testexec.CommandContext(ctx, ippCmdLine)

	stdout, err := cmdIPP.StdoutPipe()
	if err != nil {
		s.Fatal("Failed to get stdout pipe from ipp server", err)
	}

	if err := cmdIPP.Start(); err != nil {
		s.Fatal("Could not start ipp server: ", err)
	}

	outputLines = make(chan string, 100)
	stdoutScanner := bufio.NewScanner(stdout)

	go func() {
		defer close(outputLines)
		for stdoutScanner.Scan() {
			line := stdoutScanner.Text()
			outputLines <- line
		}
	}()

	address, err = waitForServerAddress(ctx, outputLines)
	if err != nil {
		s.Fatal("Could not get server address", err)
	}

	return outputLines, address
}

// GnuTLS executes the test logic
func GnuTLS(ctx context.Context, s *testing.State) {
	outputLines, serverAddress := startServer(ctx, s)
	// outputLines := make(chan string, 100)
	// serverAddress := "localhost"

	timeoutCtx, cancel := context.WithTimeout(ctx, adminTimeout)
	defer cancel()
	cmdAdmin := testexec.CommandContext(timeoutCtx, "sudo", "lpadmin", "-E", "-m", "everywhere", "-p", "printer", "-v",
		fmt.Sprintf("ipps://%s/ipp/print/ippserver-data-type-test", serverAddress))
	outBytes, err := cmdAdmin.CombinedOutput()
	out := ""
	if outBytes != nil {
		out = string(outBytes)
	}
	if err != nil {
		s.Fatalf("Could not execute admin command: %v, output: %s", err, out)
	}

	s.Log("lpadmin output: " + out)

	waitForTag(ctx, outputLines, startingTag)
	waitForTag(ctx, outputLines, encryptedTag)
}
