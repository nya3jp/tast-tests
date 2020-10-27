// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"time"

	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ScreenLatency,
		Desc: "Tests latency between pressing a key and having it shown on a screen",
		Contacts: []string{
			"mblsha@google.com",
		},
		Attr: []string{
			"group:mainline",
		},
		SoftwareDeps: []string{},
		Params:       []testing.Param{},
	})
}

const (
	keyToPress           = "m"
	keyPressCount        = 10
	cameraStartupDelay   = 410 // Manual calibration offset
	actionCaptureStarted = 1
	actionCaptureResults = 2
)

type action struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type point struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type frameData struct {
	CornerPoints [][]point `json:"cornerPoints"`
	Line         []string
}

type hostData struct {
	FramesMetaData           []frameData `json:"framesMetaData"`
	RecordingStartTimeMillis int64       `json:"recordingStartTime"`
	VideoFps                 int64       `json:"videoFps"`
	HostSyncTimestamp        int64       `json:"hostSyncTimestamp"`
}

func ScreenLatency(ctx context.Context, s *testing.State) {
	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create keyboard device: ", err)
	}

	testing.ContextLog(ctx, "Starting server")

	ln, _ := net.Listen("tcp", "127.0.0.1:")
	_, serverPort, _ := net.SplitHostPort(ln.Addr().String())
	testing.ContextLog(ctx, "Listening on address:")
	testing.ContextLog(ctx, ln.Addr())

	openAppCommand := exec.Command("adb", "shell", "am", "start", "-n",
		"com.android.example.camera2.slowmo/com.example.android.camera2.slowmo.CameraActivity",
		"--es", "port "+serverPort)
	if !runOsCommand(ctx, openAppCommand) {
		s.Fatal("Failed to start companion Android app using adb")
	}

	portForwardingCommand := exec.Command("adb", "reverse", "tcp:"+serverPort, "tcp:"+serverPort)
	if !runOsCommand(ctx, portForwardingCommand) {
		s.Fatal("Failed to initiate TCP connection with a companion Android app")
	}

	// accept connection
	conn, _ := ln.Accept()
	testing.ContextLog(ctx, "Got connection from:")
	testing.ContextLog(ctx, conn.LocalAddr().String())

	keyPressTimestamps := make([]int64, keyPressCount+1)
	var testStartTimestamp int64

	hostCommunicator := json.NewDecoder(conn)

	var hostAction action

	for {
		err := hostCommunicator.Decode(&hostAction)
		testing.ContextLog(ctx, "Decode error: ", err)
		if hostAction.Code == actionCaptureStarted {
			testStartTimestamp, keyPressTimestamps = simulateKeyPress(ctx, keyboard, keyToPress, keyPressCount)
		} else if hostAction.Code == actionCaptureResults {
			var ocrData hostData
			hostCommunicator.Decode(&ocrData)
			calculateLag(ctx, ocrData, testStartTimestamp, keyPressTimestamps)
			return
		}
	}
}

func calculateLag(ctx context.Context, ocrData hostData, testStartTimestamp int64, timestamps []int64) []int64 {
	lagResults := make([]int64, keyPressCount)
	syncOffset := ocrData.HostSyncTimestamp - testStartTimestamp

	searchKey := ""
	for i := 0; i < len(timestamps); i++ {
		searchKey += keyToPress
		found := false
		for j := 0; j < len(ocrData.FramesMetaData); j++ {
			for k := 0; k < len(ocrData.FramesMetaData[j].Line); k++ {
				if strings.HasPrefix(ocrData.FramesMetaData[j].Line[k], searchKey) {
					lagResults[i] = timestamps[i] - (ocrData.RecordingStartTimeMillis + ((int64(j) * 1000) / ocrData.VideoFps)) + syncOffset - cameraStartupDelay
					found = true
					break
				}
			}
			if found {
				testing.ContextLog(ctx, "Lag = ", lagResults[i], "ms")
				break
			}
		}
	}
	return lagResults
}

func runOsCommand(ctx context.Context, cmd *exec.Cmd) bool {
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		testing.ContextLog(ctx, fmt.Sprint(err)+": "+stderr.String())
		return false
	}
	testing.ContextLog(ctx, "Result: "+out.String())
	return true
}

func simulateKeyPress(ctx context.Context, keyboard *input.KeyboardEventWriter, key string, keyPressCount int) (int64, []int64) {
	timestamps := make([]int64, keyPressCount)
	testStartTimestamp := time.Now().UnixNano() / 1000000
	testing.Sleep(ctx, 1*time.Second)
	for i := 0; i < keyPressCount; i++ {
		keyboard.Type(ctx, key)
		timestamps[i] = time.Now().UnixNano() / 1000000
		testing.Sleep(ctx, 100*time.Millisecond)
	}
	testing.ContextLog(ctx, "Key simulation ended")
	return testStartTimestamp, timestamps
}
