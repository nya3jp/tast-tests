// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package nearbyshare is for controlling Nearby Share.
package nearbyshare

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"time"

	"chromiumos/tast/local/android/adb"
	"chromiumos/tast/testing"
)

type jsonRPCCmd struct {
	Cmd string `json:"cmd"`
	UID int    `json:"uid"`
}

type jsonRPCRequest struct {
	Method string   `json:"method"`
	ID     int      `json:"id"`
	Params []string `json:"params"`
}

type jsonRPCError struct {
	Message string `json:"message"`
}

type jsonRPCResponse struct {
	ID       int           `json:"id"`
	Result   string        `json:"result"`
	Callback *string       `json:"callback"`
	Error    *jsonRPCError `json:"error"`
}

func clientSend(s *testing.State, conn net.Conn, reqBody []byte) error {
	if n, err := conn.Write(append(reqBody, "\n"...)); err != nil {
		s.Fatalf("Failed to write, int res was %v: %v", n, err)
		return err
	} else {
		s.Log("Write success, n:", n)
	}
	return nil
}

func clientReceive(s *testing.State, conn net.Conn) ([]byte, error) {
	buf := make([]byte, 4096)
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	n, err := conn.Read(buf)
	if err != nil {
		s.Fatal("Read error: ", err)
	}
	s.Log("response n:", n)
	s.Log("raw response: ", buf[:n])
	s.Log("Raw RPC out as str: ", string(buf[:n]))

	return buf[:n], nil
}

// LaunchSnippet loads the snippet on Android device.
func LaunchSnippet(ctx context.Context, s *testing.State, testDevice *adb.Device) error {
	launchSnippetCmd := testDevice.ShellCommand(ctx,
		"am", "instrument", "--user", AndroidDefaultUser, "-w", "-e", "action", "start", NearbySnippetPackage+"/"+InstrumentationRunnerPackage)
	stdout, err := launchSnippetCmd.StdoutPipe()
	if err != nil {
		s.Fatal("Failed to create stdout pipe: ", err)
	}
	if err := launchSnippetCmd.Start(); err != nil {
		s.Fatal("Failed to start snippet on device: ", err)
		return err
	}
	reader := bufio.NewReader(stdout)

	// Check the protocol version.
	line, err := reader.ReadString('\n')
	if err != nil {
		s.Fatal("Failed to read stdout: ", err)
	}
	s.Log("First line: ", line)
	r, err := regexp.Compile("SNIPPET START, PROTOCOL ([0-9]+) ([0-9]+)")
	if err != nil {
		s.Fatal("Failed to compile regexp for protocol match: ", err)
	}
	protocolMatch := r.FindStringSubmatch(line)
	if len(protocolMatch) == 0 {
		s.Fatal("Protocol version number not found.")
	} else if protocolMatch[1] != "1" {
		s.Fatal("Incorrect protocol version (%v).", protocolMatch[1])
	}

	// Get the device port to forward to the CB.
	line, err = reader.ReadString('\n')
	if err != nil {
		s.Fatal("Failed to read stdout: ", err)
	}
	s.Log("Second line: ", line)
	r, err = regexp.Compile("SNIPPET SERVING, PORT ([0-9]+)")
	if err != nil {
		s.Fatal("Failed to compile regexp for port number match: ", err)
	}
	portMatch := r.FindStringSubmatch(line)
	if len(portMatch) == 0 {
		s.Fatal("Port number not found.")
	}
	androidPort, err := strconv.Atoi(portMatch[1])
	if err != nil {
		s.Fatal("Failed to convert port to int")
	}

	// Forward the snippet server port to the CB.
	hostPort, err := testDevice.ForwardTCP(ctx, androidPort)
	if err != nil {
		s.Fatal("Port forwarding failed: ", err)
	}
	s.Log("hostPort: ", hostPort)

	// Send a request to the snippet RPC server.
	address := fmt.Sprintf("localhost:%v", hostPort)
	conn, err := net.Dial("tcp", address)
	if err != nil {
		s.Fatal("Failed to connect to snippet server: ", err)
	}

	// Init the snippet
	// http://google3/third_party/py/mobly/controllers/android_device_lib/snippet_client.py?l=199&rcl=277322773
	reqCmd := jsonRPCCmd{UID: -1, Cmd: "initiate"}
	reqCmdBody, err := json.Marshal(&reqCmd)
	s.Log("Marshalled request: ", string(reqCmdBody))
	if err != nil {
		s.Fatal("Failed to marshal request: ", err)
		return err
	}
	clientSend(s, conn, reqCmdBody)
	clientReceive(s, conn)

	reqData := jsonRPCRequest{ID: 0, Method: "getNearbySharingVersion", Params: make([]string, 0)}
	reqBody, err := json.Marshal(&reqData)
	s.Log("Marshalled request: ", string(reqBody))
	if err != nil {
		s.Fatal("Failed to marshal request: ", err)
		return err
	}
	clientSend(s, conn, reqBody)

	// Read response.
	res := jsonRPCResponse{}
	resBytes, err := clientReceive(s, conn)
	if err != nil {
		s.Fatal("Failed to receive response from snippet server: ", err)
	}
	if err := json.Unmarshal(resBytes, &res); err != nil {
		s.Fatal("Failed to unmarshal JSON: ", err)
	}
	s.Log("RPC output: ", res)

	return nil
}

// StopSnippet stops the snippet on Android device.
func StopSnippet(ctx context.Context, s *testing.State, testDevice *adb.Device) error {
	launchSnippetCmd := testDevice.ShellCommand(ctx,
		"am", "instrument", "--user", AndroidDefaultUser, "-w", "-e", "action", "stop", NearbySnippetPackage+"/"+InstrumentationRunnerPackage)
	if err := launchSnippetCmd.Run(); err != nil {
		s.Fatal("Failed to stop snippet on device: ", err)
		return err
	}
	return nil
}
