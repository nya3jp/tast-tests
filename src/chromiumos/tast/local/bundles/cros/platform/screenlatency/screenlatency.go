// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package screenlatency contains functionality to communicate with a companion
// Android app to measure the latency between a simulated key press and when
// it's actually drawn on screen.
package screenlatency

import (
	"context"
	"encoding/json"
	"net"
	"strings"
	"time"

	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const (
	keyToPress    = "m"
	keyPressCount = 10
	// TODO(mblsha): Ideally we should investigate how to get per-frame
	// capture timestamp using the Android camera API.
	cameraStartupDelay = 0 * time.Millisecond

	// action.code must match one of these values.
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
	Lines        []string  `json:"line"`
}

type hostData struct {
	FramesMetaData []frameData `json:"framesMetaData"`
	// Time in Unix Epoch milliseconds when the recording on the Android phone started.
	RecordingStartTimeUnixMs int64 `json:"recordingStartTime"`
	VideoFPS                 int64 `json:"videoFps"`
	// Time in Unix Epoch milliseconds when the Android phone generated the |actionCaptureStarted| command.
	SentCaptureStartedActionTimeUnixMs int64 `json:"hostSyncTimestamp"`
}

// CommunicateWithCompanionApp starts listening for commands from a companion
// Android app, and calculates the latency once the OCR results are available.
func CommunicateWithCompanionApp(ctx context.Context, s *testing.State, ln net.Listener, keyboard *input.KeyboardEventWriter) {
	conn, _ := ln.Accept()
	testing.ContextLog(ctx, "Got connection from: ", conn.LocalAddr().String())

	keyPressTimestamps := make([]time.Time, keyPressCount+1)
	var testStartTime time.Time
	hostCommunicator := json.NewDecoder(conn)

	for {
		var hostAction action
		if err := hostCommunicator.Decode(&hostAction); err != nil {
			s.Fatal("Decode error: ", err)
		}

		if hostAction.Code == actionCaptureStarted {
			testStartTime, keyPressTimestamps = simulateKeyPress(ctx, keyboard, keyToPress, keyPressCount)
		} else if hostAction.Code == actionCaptureResults {
			var ocrData hostData
			hostCommunicator.Decode(&ocrData)
			calculateLag(ctx, ocrData, testStartTime, keyPressTimestamps)
			return
		} else {
			s.Fatal("Unhandled code: ", hostAction.Code)
		}
	}
}

func calculateLag(ctx context.Context, ocrData hostData, testStartTime time.Time, timestamps []time.Time) []time.Duration {
	lagResults := make([]time.Duration, keyPressCount)
	recordingStartTime := time.Unix(0, (time.Duration(ocrData.RecordingStartTimeUnixMs) * time.Millisecond).Nanoseconds())
	sentCaptureStartedTime := time.Unix(0, (time.Duration(ocrData.SentCaptureStartedActionTimeUnixMs) * time.Millisecond).Nanoseconds())
	// Account for the fact that clocks aren't synchronized between the Android phone and the DUT.
	syncOffset := testStartTime.Sub(sentCaptureStartedTime)
	frameDuration := time.Second / time.Duration(ocrData.VideoFPS)

	// timestamps contains all local timestamps when we simulated the key presses.
	searchKey := ""
	for i, ts := range timestamps {
		searchKey += keyToPress
		found := false

		// ocrData.FramesMetaData contains OCR results for each captured video frame.
		// Each frame contains info on all recognized lines of text and their coordinates.
		for j, frameData := range ocrData.FramesMetaData {
			for _, line := range frameData.Lines {
				if strings.HasPrefix(line, searchKey) {
					frameCaptureTime := recordingStartTime.Add(time.Duration(j)*frameDuration + syncOffset - cameraStartupDelay)
					lagResults[i] = ts.Sub(frameCaptureTime)
					found = true
					break
				}
			}
			if found {
				testing.ContextLog(ctx, "Lag = ", lagResults[i].Milliseconds(), "ms")
				break
			}
		}
	}
	return lagResults
}

func simulateKeyPress(ctx context.Context, keyboard *input.KeyboardEventWriter, key string, keyPressCount int) (time.Time, []time.Time) {
	timestamps := make([]time.Time, keyPressCount)

	// Wait for 1 second to account for delays before video capture is initiated.
	testStartTime := time.Now()
	testing.Sleep(ctx, 1*time.Second)

	for i := 0; i < keyPressCount; i++ {
		before := time.Now()
		// TODO(mblsha): Take into account that simulating a keypress is not instant and takes 50ms.
		keyboard.Type(ctx, key)
		timestamps[i] = time.Now()

		if timestamps[i].Sub(before).Milliseconds() < 50 {
			// Add some delay so all the key presses won't happen in a single frame.
			testing.Sleep(ctx, 50*time.Millisecond)
		}
	}

	testing.ContextLog(ctx, "Key simulation ended, waiting for OCR results")
	return testStartTime, timestamps
}
