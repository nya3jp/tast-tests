// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package mobly is for interacting with Mobly snippets on Android devices for rich Android automation controls.
// See https://github.com/google/mobly-snippet-lib for more details.
package mobly

import (
	"archive/zip"
	"bufio"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	"chromiumos/tast/common/android"
	"chromiumos/tast/common/android/adb"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// SnippetClient is a client for a single Mobly snippet running on an Android device.
// Mobly snippets run an on-device JSON RPC server, whose RPCs provide some custom controls on the Android device.
// This client can be used by Tast tests and support libraries to interact with these snippets.
type SnippetClient struct {
	device        *adb.Device
	conn          net.Conn
	listeningPort int
	hostPort      int
	requestID     int
	moblyPackage  string
}

// Constants for installing and running Mobly snippet APKs.
const (
	instrumentationRunnerClass = "com.google.android.mobly.snippet.SnippetRunner"
	protocolVersion            = "1"
)

// NewSnippetClient initializes the snippet client by doing the following:
//   1. Install and run the specified snippet APK to start the server.
//   2. Forward the snippet's listening port to the host (CrOS device) and establish a TCP connection to it.
// We can then send RPCs over the TCP connection to interact with the snippet.
// The Android package containing the Snippet class must be provided in the moblyPackage argument.
// Callers should defer Cleanup to ensure the resources used by the SnippetClient are freed.
func NewSnippetClient(ctx context.Context, d *adb.Device, moblyPackage, apkZipPath, apkName string, apkPermissions ...string) (sc *SnippetClient, err error) {
	sc = &SnippetClient{device: d, moblyPackage: moblyPackage}
	if err := unzipAndInstallApk(ctx, d, apkZipPath, apkName); err != nil {
		return sc, err
	}

	for _, p := range apkPermissions {
		permissionsCmd := sc.device.ShellCommand(ctx, "appops", "set", "--uid", moblyPackage, p, "allow")
		if err := permissionsCmd.Run(testexec.DumpLogOnError); err != nil {
			return sc, errors.Wrapf(err, "failed to grant %v permission to the snippet APK", p)
		}
	}

	// Launch the snippet APK and connect to the RPC server.
	if err := sc.launchSnippet(ctx); err != nil {
		return sc, err
	}
	defer func() {
		if err != nil {
			sc.stopSnippet(ctx)
		}
	}()

	// Forward the snippet's listening port to the host.
	if err := sc.forwardPort(ctx); err != nil {
		return sc, err
	}
	defer func() {
		if err != nil {
			sc.releasePort(ctx)
		}
	}()

	// Create a TCP connection to the snippet.
	if err := sc.tcpConn(ctx); err != nil {
		return sc, err
	}
	defer func() {
		if err != nil {
			sc.closeTCPConn(ctx)
		}
	}()
	return sc, sc.initialize(ctx)
}

// unzipAndInstallApk extracts and installs the specified APK from the given zip.
func unzipAndInstallApk(ctx context.Context, d *adb.Device, apkZipPath, apkName string) error {
	// Unzip the APK to a temp dir.
	tempDir, err := ioutil.TempDir("", "snippet-apk")
	if err != nil {
		return errors.Wrap(err, "failed to create temp dir")
	}
	defer os.RemoveAll(tempDir)

	r, err := zip.OpenReader(apkZipPath)
	if err != nil {
		return errors.Wrapf(err, "failed to unzip %v", apkZipPath)
	}
	defer r.Close()

	var apkExists bool
	for _, f := range r.File {
		if f.Name == apkName {
			src, err := f.Open()
			if err != nil {
				return errors.Wrap(err, "failed to open zip contents")
			}
			dstPath := filepath.Join(tempDir, f.Name)
			dst, err := os.Create(dstPath)
			if err != nil {
				return errors.Wrap(err, "failed to create file for copying APK")
			}
			defer dst.Close()

			if _, err := io.Copy(dst, src); err != nil {
				return errors.Wrap(err, "failed to extract apk from zip")
			}
			apkExists = true
			break
		}
	}
	if !apkExists {
		return errors.Errorf("failed to find %v in %v", apkName, apkZipPath)
	}

	if err := d.Install(ctx, filepath.Join(tempDir, apkName), adb.InstallOptionGrantPermissions); err != nil {
		return errors.Wrap(err, "failed to install snippet APK on the device")
	}

	return nil
}

// launchSnippet loads the snippet on Android device, verifies it started successfully,
// and stores its listening port that we can later forward to the host CrOS device.
func (sc *SnippetClient) launchSnippet(ctx context.Context) (err error) {
	launchSnippetCmd := sc.device.ShellCommand(ctx,
		"am", "instrument", "--user", android.DefaultUser, "-w", "-e", "action", "start", sc.moblyPackage+"/"+instrumentationRunnerClass)
	stdout, err := launchSnippetCmd.StdoutPipe()
	if err != nil {
		return errors.Wrap(err, "failed to create stdout pipe")
	}
	if err := launchSnippetCmd.Start(); err != nil {
		return errors.Wrap(err, "failed to start command to launch snippet")
	}
	defer func() {
		if err != nil {
			sc.stopSnippet(ctx)
		}
	}()

	// Confirm the protocol version and get the snippet's serving port by parsing the launch command's stdout.
	reader := bufio.NewReader(stdout)
	const (
		protocolPattern = "SNIPPET START, PROTOCOL ([0-9]+) ([0-9]+)"
		portPattern     = "SNIPPET SERVING, PORT ([0-9]+)"
	)

	line, err := reader.ReadString('\n')
	if err != nil {
		return errors.Wrap(err, "failed to read stdout while looking for the snippet protocol version")
	}
	testing.ContextLog(ctx, "snippet launch cmd stdout first line: ", line)
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
	testing.ContextLog(ctx, "snippet launch cmd stdout second line: ", line)
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
	sc.listeningPort = listeningPort
	return nil
}

// stopSnippet stops the snippet on Android device.
func (sc *SnippetClient) stopSnippet(ctx context.Context) error {
	stopSnippetCommand := sc.device.ShellCommand(ctx, "am", "instrument",
		"--user", android.DefaultUser, "-w", "-e",
		"action", "stop", sc.moblyPackage+"/"+instrumentationRunnerClass)
	if err := stopSnippetCommand.Run(); err != nil {
		return errors.Wrap(err, "failed to stop snippet on device")
	}
	return nil
}

// ReconnectToSnippet restarts a connection to the snippet on Android device.
func (sc *SnippetClient) ReconnectToSnippet(ctx context.Context) error {
	if err := sc.forwardPort(ctx); err != nil {
		return errors.Wrap(err, "port forwarding failed")
	}
	if err := sc.tcpConn(ctx); err != nil {
		return errors.Wrap(err, "failed to make tcp conn to snippet server")
	}
	if err := sc.initialize(ctx); err != nil {
		return errors.Wrap(err, "failed to reinitialize the snippet server")
	}

	return nil
}

// forwardPort forwards the snippet's listening port to the host CrOS device.
// Callers should defer releasePort to ensure it is freed on test completion or error.
func (sc *SnippetClient) forwardPort(ctx context.Context) error {
	hostPort, err := sc.device.ForwardTCP(ctx, sc.listeningPort)
	if err != nil {
		return errors.Wrap(err, "port forwarding failed")
	}
	sc.hostPort = hostPort
	return nil
}

// releasePort removes the port forwarding from the snippet's listening port to the host CrOS port.
func (sc *SnippetClient) releasePort(ctx context.Context) error {
	if err := sc.device.RemoveForwardTCP(ctx, sc.hostPort); err != nil {
		return errors.Wrap(err, "failed to remove port forwarding")
	}
	return nil
}

// tcpConn establishes a TCP connection from the host CrOS device to the snippet.
// Callers should defer closeTCPConn to ensure the connection is closed on test completion or error.
func (sc *SnippetClient) tcpConn(ctx context.Context) error {
	address := fmt.Sprintf("localhost:%v", sc.hostPort)
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return errors.Wrap(err, "failed to connect to snippet server")
	}
	sc.conn = conn
	return nil
}

// closeTCPConn closes the TCP connection from the host CrOS device to the snippet.
func (sc *SnippetClient) closeTCPConn(ctx context.Context) error {
	if err := sc.conn.Close(); err != nil {
		return errors.Wrap(err, "failed to close TCP connection to the snippet")
	}
	return nil
}

// Cleanup stops the snippet, removes port forwarding, and closes the TCP connection.
// This should be deferred after calling NewSnippetClient to ensure the resources used by the SnippetClient are released at the end of tests.
func (sc *SnippetClient) Cleanup(ctx context.Context) {
	if err := sc.closeTCPConn(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to clean up TCP connection: ", err)
	}
	if err := sc.releasePort(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to clean up port forwarding: ", err)
	}
	if err := sc.stopSnippet(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to stop snippet: ", err)
	}
}
