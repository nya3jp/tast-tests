// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package nearbysnippet is for interacting with the Nearby snippet which provides automated control of Android Nearby share.
package nearbysnippet

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/adb"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// AndroidNearbyDevice represents a connected Android device equipped with Nearby Share controls.
type AndroidNearbyDevice struct {
	device    *adb.Device
	conn      net.Conn
	requestID int
	context   *context.Context
}

// PrepareAndroidNearbyDevice initializes the specified Android device for Nearby Sharing.
// It will do the following things:
//   1. Install the Nearby snippet APK. This runs an RPC server on the device which exposes
//      APIs for automated interaction with Nearby Share.
//   2. Override the GMSCore flags required by the snippet APK. This requires adb root.
//      - If dontOverrideGMS is true, this step is skipped. This can be used to run on
//        non-rooted local devices that have other means of overriding the GMS Core flags.
//   3. Run the snippet APK.
//   4. Forward the RPC server's listening port to the host (CrOS device) and establish a TCP connection to the RPC server.
// We can then send RPCs over the TCP connection to control Nearby Share on the Android device.
func PrepareAndroidNearbyDevice(ctx context.Context, d *adb.Device, apkZipPath string, dontOverrideGMS bool) (*AndroidNearbyDevice, error) {
	// Unzip the APK to a temp dir.
	tempDir, err := ioutil.TempDir("", "snippet-apk")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create temp dir")
	}
	defer os.RemoveAll(tempDir)
	if err := testexec.CommandContext(ctx, "unzip", apkZipPath, NearbySnippetApk, "-d", tempDir).Run(testexec.DumpLogOnError); err != nil {
		return nil, errors.Wrapf(err, "failed to unzip nearby_snippet.apk from %v", apkZipPath)
	}

	// Install the snippet APK.
	pkgs, err := d.InstalledPackages(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get installed packages from Android device")
	}
	if _, ok := pkgs[nearbySnippetPackage]; !ok {
		if err := d.Install(ctx, filepath.Join(tempDir, NearbySnippetApk), adb.InstallOptionGrantPermissions); err != nil {
			return nil, errors.Wrap(err, "failed to install Nearby snippet APK on the device")
		}
		testing.ContextLog(ctx, "successfully installed the Nearby snippet on Android device")
	} else {
		testing.ContextLog(ctx, "skip installing the Nearby snippet on the Android device, already installed")
	}

	// Override the necessary GMS Core flags.
	if !dontOverrideGMS {
		if err := overrideGMSCoreFlags(ctx, d); err != nil {
			return nil, err
		}
	}

	// Launch the snippet APK and connect to the RPC server.
	c, err := launchSnippet(ctx, d)
	if err != nil {
		return nil, errors.Wrap(err, "failed to launch snippet server on device")
	}
	return &AndroidNearbyDevice{device: d, conn: *c, context: &ctx}, nil
}

// launchSnippet loads the snippet on Android device, verifies it started successfully,
// and sets up a TCP connection from the host CrOS device to the snippet APK's RPC server.
func launchSnippet(ctx context.Context, device *adb.Device) (*net.Conn, error) {
	launchSnippetCmd := device.ShellCommand(ctx,
		"am", "instrument", "--user", androidDefaultUser, "-w", "-e", "action", "start", nearbySnippetPackage+"/"+instrumentationRunnerPackage)
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
		return nil, errors.Wrap(err, "failed to read stdout while looking for the snippet protocol version")
	}
	testing.ContextLog(ctx, "Nearby Snippet launch cmd stdout first line: ", line)
	r, err := regexp.Compile(protocolPattern)
	if err != nil {
		return nil, errors.Wrap(err, "failed to compile regexp for protocol match")
	}
	protocolMatch := r.FindStringSubmatch(line)
	if len(protocolMatch) == 0 {
		return nil, errors.New("protocol version number not found in stdout")
	} else if protocolMatch[1] != nearbySnippetProtocolVersion {
		return nil, errors.Errorf("incorrect protocol version; got %v, expected %v", protocolMatch[1], nearbySnippetProtocolVersion)
	}

	// Get the device port to forward to the CB.
	line, err = reader.ReadString('\n')
	if err != nil {
		return nil, errors.Wrap(err, "failed to read stdout while looking for the snippet port")
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
	hostPort, err := device.ForwardTCP(ctx, androidPort)
	if err != nil {
		return nil, errors.Wrap(err, "port forwarding failed")
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

// overrideGMSCoreFlags overrides the GMS Core flags required by the snippet APK.
// Overriding the flags over adb requires the device to be rooted (i.e. userdebug build).
func overrideGMSCoreFlags(ctx context.Context, device *adb.Device) error {
	// Get root access.
	rootCmd := device.Command(ctx, "root")
	if err := rootCmd.Run(); err != nil {
		return errors.Wrap(err, "failed to start adb as root")
	}

	overrideCmd1 := device.ShellCommand(ctx,
		"am", "broadcast", "-a", "com.google.android.gms.phenotype.FLAG_OVERRIDE",
		"--es", "package", "com.google.android.gms.nearby",
		"--es", "user", `\*`,
		"--esa", "flags", "sharing_package_whitelist_check_bypass",
		"--esa", "values", "true",
		"--esa", "types", "boolean",
		"com.google.android.gms")
	overrideCmd2 := device.ShellCommand(ctx,
		"am", "broadcast", "-a", "com.google.android.gms.phenotype.FLAG_OVERRIDE",
		"--es", "package", "com.google.android.gms",
		"--es", "user", `\*`,
		"--esa", "flags", "GoogleCertificatesFlags__enable_debug_certificates",
		"--esa", "values", "true",
		"--esa", "types", "boolean",
		"com.google.android.gms")

	if err := overrideCmd1.Run(); err != nil {
		return errors.Wrap(err, "failed to override sharing_package_whitelist_check_bypass flag")
	}
	if err := overrideCmd2.Run(); err != nil {
		return errors.Wrap(err, "failed to override GoogleCertificatesFlags__enable_debug_certificates flag")
	}
	return nil
}

// StopSnippet stops the snippet on Android device.
func (s *AndroidNearbyDevice) StopSnippet(ctx context.Context) error {
	stopSnippetCommand := s.device.ShellCommand(ctx, "am", "instrument",
		"--user", androidDefaultUser, "-w", "-e",
		"action", "stop", nearbySnippetPackage+"/"+instrumentationRunnerPackage)
	if err := stopSnippetCommand.Run(); err != nil {
		return errors.Wrap(err, "failed to stop snippet on device")
	}
	if err := s.conn.Close(); err != nil {
		return errors.Wrap(err, "failed to close TCP connection to the snippet")
	}
	return nil
}

// jsonRPCCmd is the command format required to initialize the RPC server.
type jsonRPCCmd struct {
	Cmd string `json:"cmd"`
	UID int    `json:"uid"`
}

// jsonRPCCmdResponse is the corresponding response format to jsonRPCCmd. Only used when initializing the server.
type jsonRPCCmdResponse struct {
	Status bool `json:"status"`
	UID    int  `json:"uid"`
}

// jsonRPCRequest is the primary request format for the Nearby Share APIs.
type jsonRPCRequest struct {
	Method string        `json:"method"`
	ID     int           `json:"id"`
	Params []interface{} `json:"params"`
}

// jsonRPCRequest is the corresponding response format for jsonRPCRequest.
// The Result field's format varies depending on which method is called by
// the request, so it should be unmarshalled based on the request's API.
type jsonRPCResponse struct {
	ID       int             `json:"id"`
	Result   json.RawMessage `json:"result"`
	Callback string          `json:"callback"`
	Error    string          `json:"error"`
}

// clientSend writes a request to the RPC server. A newline is appended
// to the request body as it is required by the RPC server.
func (s *AndroidNearbyDevice) clientSend(body []byte) error {
	if _, err := s.conn.Write(append(body, "\n"...)); err != nil {
		return errors.Wrap(err, "failed to write to server")
	}
	return nil
}

// clientReceive reads the RPC server's response.
func (s *AndroidNearbyDevice) clientReceive() ([]byte, error) {
	s.conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	buf := make([]byte, 4096)
	n, err := s.conn.Read(buf)
	if err != nil {
		return nil, err
	}
	testing.ContextLog(*s.context, "Response bytes (n):", n)
	testing.ContextLog(*s.context, "Raw RPC out as str: ", string(buf[:n]))
	return buf[:n], nil
}

// clientRPCRequest formats the provided method and arguments as a jsonRPCRequest and sends it to the server.
// The server expects requests to have an incrementing ID field. The AndroidNearbyRequest struct keeps track
// of the current request ID, and it is incremented each time this function is called.
func (s *AndroidNearbyDevice) clientRPCRequest(method string, args ...interface{}) error {
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

// clientRPCResponse returns the server's response.
func (s *AndroidNearbyDevice) clientRPCResponse(lastReqID int) (jsonRPCResponse, error) {
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

// Initialize initializes the snippet RPC server.
func (s *AndroidNearbyDevice) Initialize() error {
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

// GetNearbySharingVersion retrieves the Android device's Nearby Sharing version.
func (s *AndroidNearbyDevice) GetNearbySharingVersion() (string, error) {
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
