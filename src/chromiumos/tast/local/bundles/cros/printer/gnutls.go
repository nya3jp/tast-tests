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

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: GnuTLS,
		Desc: "Validity test for lpadmin connecting to IPPS server using GnuTLS",
		Contacts: []string{
			"mw@semihalf.com",
			"skau@google.com",
			"cros-printing-dev@chromium.org",
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

// waitForTag parses the server output and waits until the tag appears.
func waitForTag(ctx context.Context, lines chan string, tag string) (line string, err error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	for {
		select {
		case <-timeoutCtx.Done():
			return "", errors.Errorf("timeout waiting for tag: %q", tag)
		case line := <-lines:
			if strings.Contains(line, tag) {
				return line, nil
			}
		}
	}
}

// waitForServerAddress obtains an address from the server output.
func waitForServerAddress(ctx context.Context, lines chan string) (string, error) {
	const serverAddressTag = "Using default listeners for "

	line, err := waitForTag(ctx, lines, serverAddressTag)
	if err != nil {
		return "", errors.Wrap(err, "failed waiting for server address")
	}

	// The ippserver log line comprising the tag ends with '.' sign,
	// which should be stripped from the output string.
	idx := strings.Index(line, serverAddressTag)
	return line[idx+len(serverAddressTag) : len(line)-1], nil
}

// startServer starts IPPS server using ippserver utility.
func startServer(ctx context.Context, s *testing.State) (address string) {
	const ippserverBinDir = "CUPS_SERVERBIN=/usr/libexec/cups"
	const ippserverCommand = "/usr/local/sbin/ippserver"
	const ippserverTestDir = "/usr/local/share/ippsample/test"

	// Launch ippserver.
	cmdIppserver := testexec.CommandContext(ctx, "env", ippserverBinDir,
		ippserverCommand, "-C", ippserverTestDir, "-r", "_print")

	// ippserver utility uses stderr for displaying logs.
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

	// Parse ippserver output and obtain its address.
	address, err = waitForServerAddress(ctx, outputLines)
	if err != nil {
		s.Fatal("Could not get server address: ", err)
	}

	return address
}

func GnuTLS(ctx context.Context, s *testing.State) {
	const lpadminCmdLine = "/usr/sbin/lpadmin"

	// Start IPPS server.
	serverCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	serverAddress := startServer(serverCtx, s)

	// Connect to the IPPS server with lpadmin.
	cmdLpadmin := testexec.CommandContext(ctx, lpadminCmdLine,
		"-E", "-m", "everywhere", "-p", "printer", "-v",
		fmt.Sprintf("ipps://%s/ipp/print/ipp-everywhere-pdf", serverAddress))
	outBytes, err := cmdLpadmin.CombinedOutput()
	if err != nil {
		out := ""
		if outBytes != nil {
			out = string(outBytes)
		}
		s.Fatalf("Could not execute lpadmin command: %v, output: %s", err, out)
	}
}
