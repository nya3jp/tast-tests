// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package nearbysnippet is for interacting with the Nearby Snippet which provides automated control of Android Nearby share.
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

	"chromiumos/tast/common/android/adb"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/android"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/testing"
)

// AndroidNearbyDevice represents a connected Android device equipped with Nearby Share controls.
// Nearby Share control is achieved by making RPCs to the Nearby Snippet running on the Android device.
// One of the RPC parameters is a requestID, which the Nearby Snippet expects to be incremented on each
// subsequent request. As a result, RPCs should not be made concurrently with this struct, as there could
// be a race condition updating requestID between requests. Requests should be made sequentially, with the
// responses processed before making the next request.
type AndroidNearbyDevice struct {
	device           *adb.Device
	conn             net.Conn
	listeningPort    int
	hostPort         int
	requestID        int
	transferCallback string
	uiDevice         *ui.Device
}

// New initializes the specified Android device for Nearby Sharing.
// It will do the following things:
//   1. Install the Nearby Snippet APK. This runs an RPC server on the device which exposes
//      APIs for automated interaction with Nearby Share.
//   2. Override the GMSCore flags required by the Nearby Snippet. This requires adb root.
//      - If overrideGMS is false, this step is skipped. This can be used to run on
//        non-rooted local devices that have other means of overriding the GMS Core flags.
//   3. Run the Nearby Snippet APK.
//   4. Forward the Nearby Snippet's listening port to the host (CrOS device) and establish a TCP connection to it.
// We can then send RPCs over the TCP connection to control Nearby Share on the Android device.
// Callers should defer Cleanup to ensure the resources used by the AndroidNearbyDevice are freed.
func New(ctx context.Context, d *adb.Device, apkZipPath string, overrideGMS bool) (a *AndroidNearbyDevice, err error) {
	a = &AndroidNearbyDevice{device: d}
	// Unzip the APK to a temp dir.
	tempDir, err := ioutil.TempDir("", "snippet-apk")
	if err != nil {
		return a, errors.Wrap(err, "failed to create temp dir")
	}
	defer os.RemoveAll(tempDir)
	if err := testexec.CommandContext(ctx, "unzip", apkZipPath, ApkName, "-d", tempDir).Run(testexec.DumpLogOnError); err != nil {
		return a, errors.Wrapf(err, "failed to unzip %v from %v", ApkName, apkZipPath)
	}

	// Install the Nearby Snippet APK.
	if err := a.device.Install(ctx, filepath.Join(tempDir, ApkName), adb.InstallOptionGrantPermissions); err != nil {
		return a, errors.Wrap(err, "failed to install Nearby Snippet APK on the device")
	}

	// Override the necessary GMS Core flags.
	if overrideGMS {
		if err := overrideGMSCoreFlags(ctx, a.device); err != nil {
			return a, err
		}
	}

	// Grant the MANAGE_EXTERNAL_STORAGE permission to the Nearby Snippet if the SDK version is 30+ (Android 11+).
	// This is required for the Android sender flow, since the Nearby Snippet sends files from external storage.
	const needsStoragePermissionsVersion = 30
	if sdkVersion, err := a.device.SDKVersion(ctx); err != nil {
		return a, errors.Wrap(err, "failed to get android sdk version")
	} else if sdkVersion >= needsStoragePermissionsVersion {
		permissionsCmd := a.device.ShellCommand(ctx, "appops", "set", "--uid", moblyPackage, "MANAGE_EXTERNAL_STORAGE", "allow")
		if err := permissionsCmd.Run(testexec.DumpLogOnError); err != nil {
			return a, errors.Wrap(err, "failed to grant external storage permissions to the Nearby Snippet APK")
		}
	}

	// Launch the Nearby Snippet APK and connect to the RPC server.
	if err := a.LaunchSnippet(ctx); err != nil {
		return a, err
	}
	defer func() {
		if err != nil {
			a.StopSnippet(ctx)
		}
	}()

	// Forward the Nearby Snippet's listening port to the host.
	if err := a.ForwardPort(ctx); err != nil {
		return a, err
	}
	defer func() {
		if err != nil {
			a.ReleasePort(ctx)
		}
	}()

	// Create a TCP connection to the Nearby Snippet.
	return a, a.TCPConn(ctx)
}

// LaunchSnippet loads the snippet on Android device, verifies it started successfully,
// and stores its listening port that we can later forward to the host CrOS device.
func (a *AndroidNearbyDevice) LaunchSnippet(ctx context.Context) (err error) {
	launchSnippetCmd := a.device.ShellCommand(ctx,
		"am", "instrument", "--user", android.DefaultUser, "-w", "-e", "action", "start", moblyPackage+"/"+instrumentationRunnerClass)
	stdout, err := launchSnippetCmd.StdoutPipe()
	if err != nil {
		return errors.Wrap(err, "failed to create stdout pipe")
	}
	if err := launchSnippetCmd.Start(); err != nil {
		return errors.Wrap(err, "failed to start command to launch Nearby Snippet")
	}
	defer func() {
		if err != nil {
			a.StopSnippet(ctx)
		}
	}()

	// Confirm the protocol version and get the Nearby Snippet's serving port by parsing the launch command's stdout.
	reader := bufio.NewReader(stdout)
	const (
		protocolPattern = "SNIPPET START, PROTOCOL ([0-9]+) ([0-9]+)"
		portPattern     = "SNIPPET SERVING, PORT ([0-9]+)"
	)

	line, err := reader.ReadString('\n')
	if err != nil {
		return errors.Wrap(err, "failed to read stdout while looking for the snippet protocol version")
	}
	testing.ContextLog(ctx, "Nearby Snippet launch cmd stdout first line: ", line)
	r, err := regexp.Compile(protocolPattern)
	if err != nil {
		return errors.Wrap(err, "failed to compile regexp for protocol match")
	}
	protocolMatch := r.FindStringSubmatch(line)
	if len(protocolMatch) == 0 {
		return errors.New("protocol version number not found in stdout")
	} else if protocolMatch[1] != protocolVersion {
		return errors.Errorf("incorrect protocol version; got %v, expected %v", protocolMatch[1], protocolVersion)
	}

	// Get the device port to forward to the CB.
	line, err = reader.ReadString('\n')
	if err != nil {
		return errors.Wrap(err, "failed to read stdout while looking for the snippet port")
	}
	testing.ContextLog(ctx, "Nearby Snippet launch cmd stdout second line: ", line)
	r, err = regexp.Compile(portPattern)
	if err != nil {
		return errors.Wrap(err, "failed to compile regexp for port number match")
	}
	portMatch := r.FindStringSubmatch(line)
	if len(portMatch) == 0 {
		return errors.New("port number not found in stdout")
	}
	listeningPort, err := strconv.Atoi(portMatch[1])
	if err != nil {
		return errors.Wrap(err, "failed to convert port to int")
	}
	a.listeningPort = listeningPort
	return nil
}

// StopSnippet stops the Nearby Snippet on Android device.
func (a *AndroidNearbyDevice) StopSnippet(ctx context.Context) error {
	stopSnippetCommand := a.device.ShellCommand(ctx, "am", "instrument",
		"--user", android.DefaultUser, "-w", "-e",
		"action", "stop", moblyPackage+"/"+instrumentationRunnerClass)
	if err := stopSnippetCommand.Run(); err != nil {
		return errors.Wrap(err, "failed to stop Nearby Snippet on device")
	}
	return nil
}

// ReconnectToSnippet restart a connection to the Neearby Snippet on Android device.
func (a *AndroidNearbyDevice) ReconnectToSnippet(ctx context.Context) error {
	if err := a.ForwardPort(ctx); err != nil {
		return errors.Wrap(err, "port forwarding failed")
	}
	if err := a.TCPConn(ctx); err != nil {
		return errors.Wrap(err, "failed to make tcp conn to snippet server")
	}
	if err := a.Initialize(ctx); err != nil {
		return errors.Wrap(err, "failed to reinitialize the snippet server")
	}

	return nil
}

// ForwardPort forwards the Nearby Snippet's listening port to the host CrOS device.
// Callers should defer ReleasePort to ensure it is freed on test completion or error.
func (a *AndroidNearbyDevice) ForwardPort(ctx context.Context) error {
	hostPort, err := a.device.ForwardTCP(ctx, a.listeningPort)
	if err != nil {
		return errors.Wrap(err, "port forwarding failed")
	}
	a.hostPort = hostPort
	return nil
}

// ReleasePort removes the port forwarding from the Nearby Snippet's listening port to the host CrOS port.
func (a *AndroidNearbyDevice) ReleasePort(ctx context.Context) error {
	if err := a.device.RemoveForwardTCP(ctx, a.hostPort); err != nil {
		return errors.Wrap(err, "failed to remove port forwarding")
	}
	return nil
}

// TCPConn establishes a TCP connection from the host CrOS device to the Nearby Snippet.
// Callers should defer CloseTCPConn to ensure the connection is closed on test completion or error.
func (a *AndroidNearbyDevice) TCPConn(ctx context.Context) error {
	address := fmt.Sprintf("localhost:%v", a.hostPort)
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return errors.Wrap(err, "failed to connect to snippet server")
	}
	a.conn = conn
	return nil
}

// CloseTCPConn closes the TCP connection from the host CrOS device to the Nearby Snippet.
func (a *AndroidNearbyDevice) CloseTCPConn(ctx context.Context) error {
	if err := a.conn.Close(); err != nil {
		return errors.Wrap(err, "failed to close TCP connection to the Nearby Snippet")
	}
	return nil
}

// Cleanup stops the Nearby Snippet, removes port forwarding, and closes the TCP connection.
// This should be deferred after calling New to ensure the resources used by the AndroidNearbyDevice are released at the end of tests.
func (a *AndroidNearbyDevice) Cleanup(ctx context.Context) {
	if err := a.CloseTCPConn(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to clean up TCP connection: ", err)
	}
	if err := a.ReleasePort(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to clean up port forwarding: ", err)
	}
	if err := a.StopSnippet(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to stop Nearby Snippet: ", err)
	}
}

// gmsOverrideCmd constructs the shell commands to override the GMS Core flags required by the Nearby Snippet.
func gmsOverrideCmd(ctx context.Context, device *adb.Device, pack, flag string) *testexec.Cmd {
	return device.ShellCommand(ctx,
		"am", "broadcast", "-a", "com.google.android.gms.phenotype.FLAG_OVERRIDE",
		"--es", "package", pack,
		"--es", "user", `\*`,
		"--esa", "flags", flag,
		"--esa", "values", "true",
		"--esa", "types", "boolean",
		"com.google.android.gms")
}

// overrideGMSCoreFlags overrides the GMS Core flags required by the Nearby Snippet.
// Overriding the flags over adb requires the device to be rooted (i.e. userdebug build).
func overrideGMSCoreFlags(ctx context.Context, device *adb.Device) error {
	// Get root access.
	rootCmd := device.Command(ctx, "root")
	if err := rootCmd.Run(); err != nil {
		return errors.Wrap(err, "failed to start adb as root")
	}

	overrideCmd1 := gmsOverrideCmd(ctx, device, "com.google.android.gms.nearby", "sharing_package_whitelist_check_bypass")
	overrideCmd2 := gmsOverrideCmd(ctx, device, "com.google.android.gms", "GoogleCertificatesFlags__enable_debug_certificates")

	if err := overrideCmd1.Run(); err != nil {
		return errors.Wrap(err, "failed to override sharing_package_whitelist_check_bypass flag")
	}
	if err := overrideCmd2.Run(); err != nil {
		return errors.Wrap(err, "failed to override GoogleCertificatesFlags__enable_debug_certificates flag")
	}
	return nil
}

// DumpLogs saves the Android device's logcat output to a file.
func (a *AndroidNearbyDevice) DumpLogs(ctx context.Context, outDir, filename string) error {
	filePath := filepath.Join(outDir, filename)
	if err := a.device.DumpLogcat(ctx, filePath); err != nil {
		testing.ContextLog(ctx, "Failed to dump Android logs: ", err)
		return errors.Wrap(err, "failed to dump Android logs")
	}
	return nil
}

// ClearLogcat clears logcat so each test run can have only relevant logs.
func (a *AndroidNearbyDevice) ClearLogcat(ctx context.Context) error {
	if err := a.device.ClearLogcat(ctx); err != nil {
		return errors.Wrap(err, "failed to clear previous logcat logs")
	}
	return nil
}

// SHA256Sum computes the sha256sum of the specified file on the Android device.
func (a *AndroidNearbyDevice) SHA256Sum(ctx context.Context, filename string) (string, error) {
	return a.device.SHA256Sum(ctx, filename)
}

// StageFile pushes the specified file to the Android device to be used in sending.
func (a *AndroidNearbyDevice) StageFile(ctx context.Context, file string) error {
	androidDst := filepath.Join(android.DownloadDir, SendDir, filepath.Base(file))
	if err := a.device.PushFile(ctx, file, androidDst); err != nil {
		return errors.Wrapf(err, "failed to push %v to %v", file, androidDst)
	}
	return nil
}

// ClearDownloads clears the device's Downloads folder, where outgoing shares are staged and incoming shares are received.
func (a *AndroidNearbyDevice) ClearDownloads(ctx context.Context) error {
	if err := a.device.RemoveContents(ctx, android.DownloadDir); err != nil {
		return errors.Wrap(err, "failed to clear Android downloads directory")
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
func (a *AndroidNearbyDevice) clientSend(body []byte) error {
	if _, err := a.conn.Write(append(body, "\n"...)); err != nil {
		return errors.Wrap(err, "failed to write to server")
	}
	return nil
}

// clientReceive reads the RPC server's response.
func (a *AndroidNearbyDevice) clientReceive(timeout time.Duration) ([]byte, error) {
	a.conn.SetReadDeadline(time.Now().Add(timeout))
	bufReader := bufio.NewReader(a.conn)
	res, err := bufReader.ReadBytes('\n')
	if err != nil {
		return nil, err
	}
	return res, nil
}

// clientRPCRequest formats the provided method and arguments as a jsonRPCRequest and sends it to the server.
// The server expects requests to have an incrementing ID field. The AndroidNearbyDevice struct keeps track
// of the current request ID, and it is incremented each time this function is called. This function returns
// the request ID that was used which callers can use when reading the response.
func (a *AndroidNearbyDevice) clientRPCRequest(ctx context.Context, method string, args ...interface{}) (int, error) {
	reqID := a.requestID
	request := jsonRPCRequest{ID: reqID, Method: method, Params: make([]interface{}, 0)}
	if len(args) > 0 {
		request.Params = args
	}
	requestBytes, err := json.Marshal(&request)
	if err != nil {
		return reqID, errors.Wrap(err, "failed to marshal request to json")
	}
	testing.ContextLog(ctx, "\tRPC request: ", string(requestBytes))

	if err := a.clientSend(requestBytes); err != nil {
		return reqID, err
	}
	a.requestID++
	return reqID, nil
}

// clientRPCResponse returns the server's response.
func (a *AndroidNearbyDevice) clientRPCResponse(ctx context.Context, lastReqID int, timeout time.Duration) (jsonRPCResponse, error) {
	var res jsonRPCResponse
	b, err := a.clientReceive(timeout)
	testing.ContextLog(ctx, "\tRPC response: ", string(b))
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

// defaultRPCResponseTimeout is the default timeout for receiving an RPC response from the Nearby Snippet.
// Most RPCs should return a response within a short amount of time. Some RPCs such as eventWaitAndGet
// and the setting getters may not respond until their specified timeouts are reached.
const defaultRPCResponseTimeout = 10 * time.Second

// Initialize initializes the Nearby Snippet.
func (a *AndroidNearbyDevice) Initialize(ctx context.Context) error {
	// Initialize the Nearby Snippet. Running the 'initiate' command with uid -1 is necessary to create a new session to the server.
	reqCmd := jsonRPCCmd{UID: -1, Cmd: "initiate"}
	reqCmdBody, err := json.Marshal(&reqCmd)
	if err != nil {
		return errors.Wrapf(err, "failed to marshal request (%+v) to json", reqCmd)
	}
	testing.ContextLog(ctx, "Initialize command request: ", string(reqCmdBody))
	if err := a.clientSend(reqCmdBody); err != nil {
		return errors.Wrap(err, "failed to send initialize command")
	}
	b, err := a.clientReceive(defaultRPCResponseTimeout)
	testing.ContextLog(ctx, "Initialize command response: ", string(b))
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
func (a *AndroidNearbyDevice) GetNearbySharingVersion(ctx context.Context) (string, error) {
	id, err := a.clientRPCRequest(ctx, "getNearbySharingVersion")
	if err != nil {
		return "", err
	}
	// Read response.
	res, err := a.clientRPCResponse(ctx, id, defaultRPCResponseTimeout)
	if err != nil {
		return "", err
	}

	var version string
	if err := json.Unmarshal(res.Result, &version); err != nil {
		return "", errors.Wrap(err, "failed to parse version number from json result")
	}
	return version, nil
}

// settingTimeoutSeconds is the time to wait for the Nearby Snippet to return settings values.
// Only used by getDeviceName, getDataUsage, and getVisibility RPCs.
const settingTimeoutSeconds = 10

// GetDeviceName retrieve's the Android device's display name for Nearby Share.
func (a *AndroidNearbyDevice) GetDeviceName(ctx context.Context) (string, error) {
	id, err := a.clientRPCRequest(ctx, "getDeviceName", settingTimeoutSeconds)
	if err != nil {
		return "", err
	}
	// Read response.
	res, err := a.clientRPCResponse(ctx, id, settingTimeoutSeconds*time.Second)
	if err != nil {
		return "", err
	}

	var name string
	if err := json.Unmarshal(res.Result, &name); err != nil {
		return "", errors.Wrap(err, "failed to parse device name from json result")
	}
	return name, nil
}

// GetDataUsage retrieve's the Android device's Nearby Share data usage setting.
func (a *AndroidNearbyDevice) GetDataUsage(ctx context.Context) (DataUsage, error) {
	var data DataUsage
	id, err := a.clientRPCRequest(ctx, "getDataUsage", settingTimeoutSeconds)
	if err != nil {
		return data, err
	}
	// Read response.
	res, err := a.clientRPCResponse(ctx, id, settingTimeoutSeconds*time.Second)
	if err != nil {
		return data, err
	}

	if err := json.Unmarshal(res.Result, &data); err != nil {
		return data, errors.Wrap(err, "failed to parse data usage from json result")
	}
	return data, nil
}

// GetVisibility retrieve's the Android device's Nearby Share visibility setting.
func (a *AndroidNearbyDevice) GetVisibility(ctx context.Context) (Visibility, error) {
	var vis Visibility
	id, err := a.clientRPCRequest(ctx, "getVisibility", settingTimeoutSeconds)
	if err != nil {
		return vis, err
	}
	// Read response.
	res, err := a.clientRPCResponse(ctx, id, settingTimeoutSeconds*time.Second)
	if err != nil {
		return vis, err
	}

	if err := json.Unmarshal(res.Result, &vis); err != nil {
		return vis, errors.Wrap(err, "failed to parse device visibility from json result")
	}
	return vis, nil
}

// SetupDevice configures the Android device's Nearby Share settings.
func (a *AndroidNearbyDevice) SetupDevice(ctx context.Context, dataUsage DataUsage, visibility Visibility, name string) error {
	id, err := a.clientRPCRequest(ctx, "setupDevice", dataUsage, visibility, name)
	if err != nil {
		return err
	}
	_, err = a.clientRPCResponse(ctx, id, defaultRPCResponseTimeout)
	return err
}

// eventWaitAndGet waits for the specified event associated with the RPC that returned callbackID to appear in the snippet's event cache.
func (a *AndroidNearbyDevice) eventWaitAndGet(ctx context.Context, callbackID string, eventName SnippetEvent, timeout time.Duration) (jsonRPCResponse, error) {
	var res jsonRPCResponse
	id, err := a.clientRPCRequest(ctx, "eventWaitAndGet", callbackID, eventName, int(timeout.Milliseconds()))
	if err != nil {
		return res, err
	}
	// Read response with a slightly extended timeout. eventWaitAndGet won't respond until the event is posted in the snippet cache,
	// or the timeout is reached. In the timeout case, we need to set the TCP read deadline a little later so we'll get the response before the conn times out.
	return a.clientRPCResponse(ctx, id, timeout+time.Second)
}

// ReceiveFile starts receiving with a timeout.
// Sets the AndroidNearbyDevice's transferCallback, which is needed when awaiting follow-up SnippetEvents when calling eventWaitAndGet.
func (a *AndroidNearbyDevice) ReceiveFile(ctx context.Context, senderName, receiverName string, isHighVisibility bool, turnaroundTime time.Duration) error {
	// Reset the transferCallback between shares.
	a.transferCallback = ""
	id, err := a.clientRPCRequest(ctx, "receiveFile", senderName, receiverName, isHighVisibility, int(turnaroundTime.Seconds()))
	if err != nil {
		return err
	}
	res, err := a.clientRPCResponse(ctx, id, defaultRPCResponseTimeout)
	if err != nil {
		return err
	}
	a.transferCallback = res.Callback
	return nil
}

// AwaitReceiverAccept should be used to wait for the onAwaitingReceiverAccept SnippetEvent, which indicates
// that the Android sender has successfully connected to the receiver. The response includes the secure connection token.
func (a *AndroidNearbyDevice) AwaitReceiverAccept(ctx context.Context, timeout time.Duration) (string, error) {
	if a.transferCallback == "" {
		return "", errors.New("transferCallback is not set, a share needs to be initiated first")
	}
	res, err := a.eventWaitAndGet(ctx, a.transferCallback, SnippetEventOnAwaitingReceiverAccept, timeout)
	if err != nil {
		return "", errors.Wrap(err, "failed waiting for onAwaitingReceiverAccept event to know that Android sender has connected to receiver")
	}
	// Sample response: {"result":{"callbackId":"1-4","name":"onAwaitingReceiverAccept","data":{"token":"3365","connectionTimeMs":X}},...}
	// Unmarshall 'result' to a map instead of building a matching struct just to get one value.
	var result map[string]interface{}
	if err := json.Unmarshal(res.Result, &result); err != nil {
		return "", errors.Wrap(err, "failed to read result map from json response")
	}

	// The token value is in another map called 'data' within the 'result' map.
	data, ok := result["data"]
	if !ok {
		return "", errors.Wrap(err, "'data' map didn't exist in onAwaitingReceiverAccept response's result field")
	}
	token := data.(map[string]interface{})["token"]
	tokenStr, ok := token.(string)
	if !ok {
		return "", errors.Wrap(err, "share token in onAwaitingReceiverAccept response was not a string")
	}
	return tokenStr, nil
}

// AwaitReceiverConfirmation should be used after ReceiveFile to wait for the onLocalConfirmation SnippetEvent, which indicates
// that the Android device has detected the incoming share and is awaiting confirmation to begin the transfer.
func (a *AndroidNearbyDevice) AwaitReceiverConfirmation(ctx context.Context, timeout time.Duration) error {
	if a.transferCallback == "" {
		return errors.New("transferCallback is not set, ReceiveFile should be executed first")
	}
	if _, err := a.eventWaitAndGet(ctx, a.transferCallback, SnippetEventOnLocalConfirmation, timeout); err != nil {
		return errors.Wrap(err, "failed waiting for onLocalConfirmation event to know that Android is ready to start the transfer")
	}
	return nil
}

// AwaitSharingStopped waits for the onStop event, which indicates that sharing has stopped and Android Nearby Share teardown
// tasks have been completed. It does not necessarily indicate that the transfer succeeded.
func (a *AndroidNearbyDevice) AwaitSharingStopped(ctx context.Context, timeout time.Duration) error {
	if a.transferCallback == "" {
		return errors.New("transferCallback is not set, a share needs to be initiated first")
	}
	if _, err := a.eventWaitAndGet(ctx, a.transferCallback, SnippetEventOnStop, timeout); err != nil {
		return errors.Wrap(err, "failed waiting for onStop event to know that transfer is complete on Android")
	}
	return nil
}

// AcceptTheSharing accepts the share on the receiver side.
func (a *AndroidNearbyDevice) AcceptTheSharing(ctx context.Context, token string) error {
	id, err := a.clientRPCRequest(ctx, "acceptTheSharing", token)
	if err != nil {
		return err
	}
	_, err = a.clientRPCResponse(ctx, id, defaultRPCResponseTimeout)
	return err
}

// CancelReceivingFile ends Nearby Share on the receiving side. This is used to fail fast instead of waiting for ReceiveFile's timeout.
func (a *AndroidNearbyDevice) CancelReceivingFile(ctx context.Context) error {
	id, err := a.clientRPCRequest(ctx, "cancelReceivingFile")
	if err != nil {
		return err
	}
	// Read response.
	_, err = a.clientRPCResponse(ctx, id, defaultRPCResponseTimeout)
	return err
}

// SendFile starts sending with a timeout.
// Sets the AndroidNearbyDevice's transferCallback, which is needed when awaiting follow-up SnippetEvents when calling eventWaitAndGet.
func (a *AndroidNearbyDevice) SendFile(ctx context.Context, senderName, receiverName, shareFileName string, mimetype MimeType, turnaroundTime time.Duration) error {
	// Reset the transferCallback between shares.
	a.transferCallback = ""
	id, err := a.clientRPCRequest(ctx, "sendFile", senderName, receiverName, shareFileName, mimetype, int(turnaroundTime.Seconds()))
	if err != nil {
		return err
	}
	res, err := a.clientRPCResponse(ctx, id, defaultRPCResponseTimeout)
	if err != nil {
		return err
	}
	a.transferCallback = res.Callback
	return nil
}

// CancelSendingFile ends Nearby Share on the sending side. This is used to fail fast instead of waiting for SendFile's timeout.
func (a *AndroidNearbyDevice) CancelSendingFile(ctx context.Context) error {
	id, err := a.clientRPCRequest(ctx, "cancelSendingFile")
	if err != nil {
		return err
	}
	// Read response.
	_, err = a.clientRPCResponse(ctx, id, defaultRPCResponseTimeout)
	return err
}

// InitUI initializes a UI automator connection to the Android device. Callers should defer CloseUI to free the associated resources.
func (a *AndroidNearbyDevice) InitUI(ctx context.Context) error {
	d, err := ui.NewDevice(ctx, a.device)
	if err != nil {
		return errors.Wrap(err, "failed initializing UI automator")
	}
	a.uiDevice = d
	return nil
}

// CloseUI closes the UI automator connection.
func (a *AndroidNearbyDevice) CloseUI(ctx context.Context) error {
	return a.uiDevice.Close(ctx)
}

// WaitForInContactSenderUI waits for the sharing UI that appears when there is an incoming share from a contact.
func (a *AndroidNearbyDevice) WaitForInContactSenderUI(ctx context.Context, sender string, timeout time.Duration) error {
	senderText := a.uiDevice.Object(ui.ID("com.google.android.gms:id/title"))
	return senderText.WaitForText(ctx, sender, timeout)
}

// AcceptUI accepts the incoming contacts share through the UI and waits for the share to finish by waiting for the receiving UI to be gone.
func (a *AndroidNearbyDevice) AcceptUI(ctx context.Context, timeout time.Duration) error {
	acceptBtn := a.uiDevice.Object(ui.ID("com.google.android.gms:id/accept_btn"))
	if err := acceptBtn.WaitForExists(ctx, 10*time.Second); err != nil {
		return errors.Wrap(err, "failed waiting for Accept button to exist")
	}
	if err := acceptBtn.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click Accept button")
	}
	receiveCard := a.uiDevice.Object(ui.ID("com.google.android.gms:id/card"))
	if err := receiveCard.WaitUntilGone(ctx, timeout); err != nil {
		return errors.Wrap(err, "failed waiting for receive UI to be gone")
	}
	return nil
}

// AndroidAttributes contains information about the Android device and its settings that are relevant to Nearby Share.
// "Android" is redundantly prepended to the field names to make them easy to distinguish from CrOS attributes in test logs.
type AndroidAttributes struct {
	DisplayName         string
	User                string
	DataUsage           string
	Visibility          string
	NearbyShareVersion  string
	GMSCoreVersion      int
	AndroidVersion      int
	SDKVersion          int
	ProductName         string
	ModelName           string
	DeviceName          string
	BluetoothMACAddress string
}

// GetAndroidAttributes returns the AndroidAttributes for the device.
func (a *AndroidNearbyDevice) GetAndroidAttributes(ctx context.Context) (*AndroidAttributes, error) {
	var metadata AndroidAttributes
	displayName, err := a.GetDeviceName(ctx)
	if err != nil {
		return nil, err
	}
	metadata.DisplayName = displayName

	user, err := a.device.GoogleAccount(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get device user account")
	}
	metadata.User = user

	dataUsage, err := a.GetDataUsage(ctx)
	if err != nil {
		return nil, err
	}
	if val, ok := DataUsageStrings[dataUsage]; ok {
		metadata.DataUsage = val
	} else {
		return nil, errors.Errorf("undefined dataUsage: %v", dataUsage)
	}

	visibility, err := a.GetVisibility(ctx)
	if err != nil {
		return nil, err
	}
	if val, ok := VisibilityStrings[visibility]; ok {
		metadata.Visibility = val
	} else {
		return nil, errors.Errorf("undefined visibility: %v", visibility)
	}

	nearbyVersion, err := a.GetNearbySharingVersion(ctx)
	if err != nil {
		return nil, err
	}
	metadata.NearbyShareVersion = nearbyVersion

	gmsVersion, err := a.device.GMSCoreVersion(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get GMS Core version")
	}
	metadata.GMSCoreVersion = gmsVersion

	androidVersion, err := a.device.AndroidVersion(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get Android version")
	}
	metadata.AndroidVersion = androidVersion

	sdkVersion, err := a.device.SDKVersion(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get Android SDK version")
	}
	metadata.SDKVersion = sdkVersion

	metadata.ProductName = a.device.Product
	metadata.ModelName = a.device.Model
	metadata.DeviceName = a.device.Device

	mac, err := a.device.BluetoothMACAddress(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the Bluetooth MAC address")
	}
	metadata.BluetoothMACAddress = mac

	return &metadata, nil
}
