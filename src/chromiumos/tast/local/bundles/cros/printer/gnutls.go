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
	ippserverBinDir  = "CUPS_SERVERBIN=/usr/libexec/cups"
	ippserverCommand = "/usr/sbin/ippserver"
	ippserverTestDir = "/usr/local/share/ippsample/test"

	lpadminCmdLine = "/usr/sbin/lpadmin"

	serverAddressTag = "Using default listeners for "

	serverTagTimeout = 5 * time.Second
)

func init() {
	testing.AddTest(&testing.Test{
		Func: GnuTLS,
		Desc: "Validity test for lpadmin connecting to IPPS server using GnuTLS",
		Contacts: []string{
			"mw@semihalf.com",
		},
		SoftwareDeps: []string{"cros_internal", "cups"},
		Attr: []string{
			"group:mainline",
			"group:paper-io",
			"paper-io_printing",
			"informational",
		},
	})
}

// waitForTag - helper routine for parsing output messages
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

// waitForServerAddress - obtain address from server output
func waitForServerAddress(ctx context.Context, lines chan string) (string, error) {

	line, err := waitForTag(ctx, lines, serverAddressTag)
	if err != nil {
		return "", errors.Wrap(err, "failed waiting for server address")
	}

	idx := strings.Index(line, serverAddressTag)
	return line[idx+len(serverAddressTag) : len(line)-1], nil
}

// startServer - start IPPS server using ippserver utility
func startServer(ctx context.Context, s *testing.State) (address string) {

	// Launch ippserver
	cmdIppserver := testexec.CommandContext(ctx, "env", ippserverBinDir,
		ippserverCommand, "-C", ippserverTestDir, "-r", "_print")

	// ippserver utility uses stderr for displaying logs
	stderr, err := cmdIppserver.StderrPipe()
	if err != nil {
		s.Fatal("Failed to get stderr pipe from ippserver: ", err)
	}

	if err := cmdIppserver.Start(); err != nil {
		s.Fatal("Could not start ipp server: ", err)
	}

	outputLines := make(chan string, 100)
	stderrScanner := bufio.NewScanner(stderr)

	go func() {
		defer close(outputLines)
		for stderrScanner.Scan() {
			line := stderrScanner.Text()
			outputLines <- line
		}
	}()

	// Parse ippserver output and obtain its address
	address, err = waitForServerAddress(ctx, outputLines)
	if err != nil {
		s.Fatal("Could not get server address: ", err)
	}

	return address
}

// GnuTLS main test logic
func GnuTLS(ctx context.Context, s *testing.State) {

	// Start IPPS server
	serverAddress := startServer(ctx, s)

	// Connect to the IPPS server with lpadmin
	cmdLpadmin := testexec.CommandContext(ctx, lpadminCmdLine,
		"-E", "-m", "everywhere", "-p", "printer", "-v",
		fmt.Sprintf("ipps://%s/ipp/print/ipp-everywhere-pdf", serverAddress))
	outBytes, err := cmdLpadmin.CombinedOutput()
	out := ""
	if outBytes != nil {
		out = string(outBytes)
	}
	if err != nil {
		s.Fatalf("Could not execute lpadmin command: %v, output: %s", err, out)
	}
}
