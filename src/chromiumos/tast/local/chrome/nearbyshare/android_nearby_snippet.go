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

	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/adb"
	"chromiumos/tast/testing"
)

type SnippetDevice struct {
	device    *adb.Device
	conn      net.Conn
	requestID int
	context   *context.Context
}

func PrepareSnippetDevice(ctx context.Context, d *adb.Device, apkPath string) (*SnippetDevice, error) {
	pkgs, err := d.InstalledPackages(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get installed packages from Android device")
	}
	if _, ok := pkgs[NearbySnippetPackage]; !ok {
		if err := d.Install(ctx, apkPath, adb.InstallOptionGrantPermissions); err != nil {
			return nil, errors.Wrap(err, "failed to install nearby snippet APK to the device")
		}
		testing.ContextLog(ctx, "Successfully install the Nearby snippet to Android device!")
	} else {
		testing.ContextLog(ctx, "Skip installing the Nearby snippet to Android device, already installed!")
	}

	c, err := LaunchSnippet(ctx, d)
	if err != nil {
		return nil, errors.Wrap(err, "failed to launch snippet server on device")
	}
	return &SnippetDevice{device: d, conn: *c, context: &ctx}, nil
}

// LaunchSnippet loads the snippet on Android device.
func LaunchSnippet(ctx context.Context, testDevice *adb.Device) (*net.Conn, error) {
	// Get root access.
	rootCmd := testDevice.Command(ctx, "root")
	if err := rootCmd.Run(); err != nil {
		return nil, err
	}

	// Override GMS core flags for Nearby Share.
	overrideCmd1 := testDevice.ShellCommand(ctx,
		"am", "broadcast", "-a", "com.google.android.gms.phenotype.FLAG_OVERRIDE",
		"--es", "package", "com.google.android.gms.nearby",
		"--es", "user", `\*`,
		"--esa", "flags", "sharing_package_whitelist_check_bypass",
		"--esa", "values", "true",
		"--esa", "types", "boolean",
		"com.google.android.gms")
	overrideCmd2 := testDevice.ShellCommand(ctx,
		"am", "broadcast", "-a", "com.google.android.gms.phenotype.FLAG_OVERRIDE",
		"--es", "package", "com.google.android.gms",
		"--es", "user", `\*`,
		"--esa", "flags", "GoogleCertificatesFlags__enable_debug_certificates",
		"--esa", "values", "true",
		"--esa", "types", "boolean",
		"com.google.android.gms")

	if err := overrideCmd1.Run(); err != nil {
		return nil, err
	}
	if err := overrideCmd2.Run(); err != nil {
		return nil, err
	}

	launchSnippetCmd := testDevice.ShellCommand(ctx,
		"am", "instrument", "--user", AndroidDefaultUser, "-w", "-e", "action", "start", NearbySnippetPackage+"/"+InstrumentationRunnerPackage)
	stdout, err := launchSnippetCmd.StdoutPipe()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create stdout pipe")
	}
	if err := launchSnippetCmd.Start(); err != nil {
		return nil, errors.Wrap(err, "failed to start command to launch snippet server")
	}

	// Confirm the protocol version and get the snippet's serving port by parsing the launch command's stdout.
	reader := bufio.NewReader(stdout)
	const (
		protocolPattern = "SNIPPET START, PROTOCOL ([0-9]+) ([0-9]+)"
		portPattern     = "SNIPPET SERVING, PORT ([0-9]+)"
	)

	line, err := reader.ReadString('\n')
	if err != nil {
		return nil, errors.Wrap(err, "failed to read stdout")
	}
	testing.ContextLog(ctx, "Nearby Snippet launch cmd stdout first line: ", line)
	r, err := regexp.Compile(protocolPattern)
	if err != nil {
		return nil, errors.Wrap(err, "failed to compile regexp for protocol match")
	}
	protocolMatch := r.FindStringSubmatch(line)
	if len(protocolMatch) == 0 {
		return nil, errors.New("protocol version number not found in stdout")
	} else if protocolMatch[1] != NearbySnippetProtocolVersion {
		return nil, errors.Errorf("incorrect protocol version; got %v, expected %v", protocolMatch[1], NearbySnippetProtocolVersion)
	}

	// Get the device port to forward to the CB.
	line, err = reader.ReadString('\n')
	if err != nil {
		return nil, errors.Wrap(err, "failed to read stdout")
	}
	testing.ContextLog(ctx, "Nearby Snippet launch cmd stdout second line: ", line)
	r, err = regexp.Compile(portPattern)
	if err != nil {
		return nil, errors.Wrap(err, "failed to compile regexp for port number match")
	}
	portMatch := r.FindStringSubmatch(line)
	if len(portMatch) == 0 {
		return nil, errors.New("port number not found in stdout")
	}
	androidPort, err := strconv.Atoi(portMatch[1])
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert port to int")
	}

	// Forward the snippet server port to the CB.
	hostPort, err := testDevice.ForwardTCP(ctx, androidPort)
	if err != nil {
		return nil, errors.Wrap(err, "port forwarding failed: ")
	}
	testing.ContextLog(ctx, "hostPort: ", hostPort)

	// Set up a TCP connection to the RPC server.
	address := fmt.Sprintf("localhost:%v", hostPort)
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to snippet server")
	}
	return &conn, nil
}

type jsonRPCCmd struct {
	Cmd string `json:"cmd"`
	UID int    `json:"uid"`
}

type jsonRPCCmdResponse struct {
	Status bool `json:"status"`
	UID    int  `json:"uid"`
}

type jsonRPCRequest struct {
	Method string        `json:"method"`
	ID     int           `json:"id"`
	Params []interface{} `json:"params"`
}

// type jsonRPCResponse struct {
// 	ID       int    `json:"id"`
// 	Result   string `json:"result"`
// 	Callback string `json:"callback"`
// 	Error    string `json:"error"`
// }

type jsonRPCResponse struct {
	ID       int             `json:"id"`
	Result   json.RawMessage `json:"result"`
	Callback string          `json:"callback"`
	Error    string          `json:"error"`
}

func (s *SnippetDevice) clientSend(body []byte) error {
	if _, err := s.conn.Write(append(body, "\n"...)); err != nil {
		return errors.Wrap(err, "failed to write to server")
	}
	return nil
}

func (s *SnippetDevice) clientReceive() ([]byte, error) {
	s.conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	buf := make([]byte, 4096)
	n, err := s.conn.Read(buf)
	if err != nil {
		return nil, err
	}
	testing.ContextLog(*s.context, "response n:", n)
	testing.ContextLog(*s.context, "raw response: ", buf[:n])
	testing.ContextLog(*s.context, "Raw RPC out as str: ", string(buf[:n]))
	return buf[:n], nil
	// return ioutil.ReadAll(s.conn)
}

func (s *SnippetDevice) clientRPCRequest(method string, args ...interface{}) error {
	request := jsonRPCRequest{ID: s.requestID, Method: method, Params: make([]interface{}, 0)}
	if len(args) > 0 {
		request.Params = args
	}
	requestBytes, err := json.Marshal(&request)
	if err != nil {
		return errors.Wrap(err, "failed to marshal request to json")
	}
	testing.ContextLog(*s.context, "Marshalled request: ", string(requestBytes))

	if err := s.clientSend(requestBytes); err != nil {
		return err
	}
	s.requestID++
	return nil
}

func (s *SnippetDevice) clientRPCResponse(lastReqID int) (jsonRPCResponse, error) {
	var res jsonRPCResponse
	b, err := s.clientReceive()
	if err != nil {
		return res, err
	}
	if err := json.Unmarshal(b, &res); err != nil {
		return res, err
	}
	if res.Error != "" {
		return res, errors.Errorf("response error %v", res.Error)
	}
	if res.ID != lastReqID {
		return res, errors.Errorf("response ID mismatch; expected %v, got %v", lastReqID, res.ID)
	}
	return res, nil
}

// InitializeSnippet initializes the snippet RPC server.
func (s *SnippetDevice) Initialize() error {
	// Initialize the snippet server.
	reqCmd := jsonRPCCmd{UID: -1, Cmd: "initiate"}
	reqCmdBody, err := json.Marshal(&reqCmd)
	if err != nil {
		return errors.Wrapf(err, "failed to marshal request (%+v) to json", reqCmd)
	}
	testing.ContextLog(*s.context, "Marshalled initialize command: ", string(reqCmdBody))
	if err := s.clientSend(reqCmdBody); err != nil {
		return errors.Wrap(err, "failed to send initialize command")
	}
	b, err := s.clientReceive()
	if err != nil {
		return errors.Wrap(err, "failed to read response to initialize command")
	}
	testing.ContextLog(*s.context, "initialize command response: ", string(b))

	// Unmarshal the response and check if the initialize command was successful.
	var res jsonRPCCmdResponse
	if err := json.Unmarshal(b, &res); err != nil {
		return errors.Wrap(err, "failed to unmarshal initialize command response")
	}
	if !res.Status {
		return errors.New("snippet RPC initialize command did not succeed")
	}
	return nil
}

// StopSnippet stops the snippet on Android device.
func (s *SnippetDevice) StopSnippet(ctx context.Context) error {
	stopSnippetCommand := s.device.ShellCommand(ctx,
		"am", "instrument", "--user", AndroidDefaultUser, "-w", "-e", "action", "stop", NearbySnippetPackage+"/"+InstrumentationRunnerPackage)
	if err := stopSnippetCommand.Run(); err != nil {
		return errors.Wrap(err, "failed to stop snippet on device")
	}
	s.conn.Close()
	return nil
}

func (s *SnippetDevice) GetNearbySharingVersion() (string, error) {
	if err := s.clientRPCRequest("getNearbySharingVersion"); err != nil {
		return "", err
	}
	// Read response.
	res, err := s.clientRPCResponse(s.requestID - 1)
	if err != nil {
		return "", err
	}

	var version string
	if err := json.Unmarshal(res.Result, &version); err != nil {
		return "", errors.Wrap(err, "failed to parse version number from json result")
	}
	return version, nil
}

func (s *SnippetDevice) SetupDevice(dataUsage SnippetDataUsage, visibility SnippetVisibility, name string) error {
	if err := s.clientRPCRequest("setupDevice", dataUsage, visibility, name); err != nil {
		return err
	}
	// Read response.
	_, err := s.clientRPCResponse(s.requestID - 1)
	return err
}

// func (s *SnippetDevice) ReceiveFile(callbackID, senderName, receiverName string, turnaroundTime time.Duration) (string, error) {
func (s *SnippetDevice) ReceiveFile(senderName, receiverName string, turnaroundTime time.Duration) (string, error) {
	if err := s.clientRPCRequest("receiveFile", senderName, receiverName, int(turnaroundTime.Seconds())); err != nil {
		return "", err
	}
	// Read response.
	res, err := s.clientRPCResponse(s.requestID - 1)
	if err != nil {
		return "", err
	}
	return res.Callback, nil
}

func (s *SnippetDevice) EventWaitAndGet(callbackID string, eventName SnippetEvent, timeout time.Duration) error {
	if err := s.clientRPCRequest("eventWaitAndGet", callbackID, eventName, int(timeout.Milliseconds())); err != nil {
		return err
	}
	// Read response.
	_, err := s.clientRPCResponse(s.requestID - 1)
	return err
}

func (s *SnippetDevice) AcceptTheSharing(token string) error {
	if err := s.clientRPCRequest("acceptTheSharing", token); err != nil {
		return err
	}
	// Read response.
	_, err := s.clientRPCResponse(s.requestID - 1)
	return err
}

func (s *SnippetDevice) CancelReceivingFile() error {
	if err := s.clientRPCRequest("cancelReceivingFile"); err != nil {
		return err
	}
	// Read response.
	_, err := s.clientRPCResponse(s.requestID - 1)
	return err
}
