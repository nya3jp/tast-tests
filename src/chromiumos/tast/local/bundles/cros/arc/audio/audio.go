// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package audio contains common utilities to help writing ARC audio tests.
package audio

import (
	"bufio"
	"context"
	"strings"
	"sync"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// TestParameters holds the arc audio tast parameters.
type TestParameters struct {
	Permission string
	Class      string
}

const (
	apk = "ArcAudioTest.apk"
	pkg = "org.chromium.arc.testapp.arcaudiotestapp"

	// UI IDs in the app.
	idPrefix = pkg + ":id/"
	resultID = idPrefix + "test_result"
	logID    = idPrefix + "test_result_log"
)

// ArcTast holds the resource of arc audio tast tests.
type ArcTast struct {
	arc    *arc.ARC
	cr     *chrome.Chrome
	device *ui.Device
	tconn  *chrome.TestConn
}

// KeyValue is a map of string type key and string type value.
type KeyValue map[string]string

// DumpActiveStreams parse active stream params from "cras_test_client --dump_audio_thread" log.
func DumpActiveStreams(ctx context.Context) ([]KeyValue, string, error) {
	dump, err := testexec.CommandContext(ctx, "cras_test_client", "--dump_audio_thread").Output()
	if err != nil {
		return nil, "", errors.Errorf("failed to dump audio thread: %s", err)
	}

	s := strings.Split(string(dump), "-------------stream_dump------------")
	if len(s) < 2 {
		return nil, string(dump), errors.New("no stream_dump")
	}
	s = strings.Split(s[1], "Audio Thread Event Log:")
	if len(s) == 0 {
		return nil, string(dump), errors.New("invalid stream_dump")
	}
	streamStr := strings.Trim(s[0], " \n\t")
	streams := make([]KeyValue, 0)

	// No active streams, return empty slice.
	if streamStr == "" {
		return streams, streamStr, nil
	}
	scanner := bufio.NewScanner(strings.NewReader(streamStr))
	stream := make(KeyValue)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			// Appends a stream when sees an empty line.
			streams = append(streams, stream)
			stream = make(map[string]string)
			continue
		}
		pair := strings.Split(line, ":")
		k := strings.Trim(pair[0], " ")
		v := strings.Trim(pair[1], " ")
		stream[k] = v
	}
	// Appends the last stream
	streams = append(streams, stream)

	return streams, streamStr, nil
}

// Run01ResultTest runs the test that result can be either '0' or '1' on the test App UI, where '0' means fail and '1'
// means pass.
func Run01ResultTest(ctx context.Context, s *testing.State) {
	atast, err := NewArcTast(ctx, s)
	if err != nil {
		s.Fatal("Failed to init test case: ", err)
	}
	defer atast.Close()

	s.Log("Installing app")
	if err := atast.InstallAPK(ctx, s.DataPath(apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}
	s.Log("Starting test activity")
	param := s.Param().(TestParameters)
	act, err := atast.StartActivity(ctx, param)
	if err != nil {
		s.Fatal("Failed runing test case: ", err)
	}
	defer act.Close()
	s.Log("Verifying test result")
	ok, reason, err := atast.Verify01Result(ctx)
	if err != nil {
		s.Fatal("Failed verifying test case: ", err)
	}
	if !ok {
		s.Error("Test failed: ", reason)
	}
}

// Run01withPollResultTest runs the test that verifies the '0' or '1' result on the test App UI, where '0' means fail and '1'
// means pass and also starts a goroutine to verifies the polling result as well. Both the App result and polling results need to be passed.
func Run01withPollResultTest(ctx context.Context, s *testing.State, f func(context.Context) error, timeout time.Duration) {
	atast, err := NewArcTast(ctx, s)
	if err != nil {
		s.Fatal("Failed to init test case: ", err)
	}
	defer atast.Close()

	s.Log("Installing app")
	if err := atast.InstallAPK(ctx, s.DataPath(apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	// Starts a goroutine to verifies the polling result.
	// The pollFunc can fail the test case by s.Error()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := testing.Poll(ctx, f, &testing.PollOptions{Timeout: timeout}); err != nil {
			s.Error("Polling result failed: ", err)
		}
	}()

	s.Log("Starting test activity")
	param := s.Param().(TestParameters)
	act, err := atast.StartActivity(ctx, param)
	if err != nil {
		s.Fatal("Failed runing test case: ", err)
	}
	defer act.Close()
	s.Log("Verifying app UI result")
	ok, reason, err := atast.Verify01Result(ctx)
	if err != nil {
		s.Fatal("Failed verifying app UI result: ", err)
	}
	if !ok {
		s.Error("Test failed: ", reason)
	}

	// Waits until goroutine finish verifying poll result.
	wg.Wait()
}

// NewArcTast creates an `ArcAudio`.
func NewArcTast(ctx context.Context, s *testing.State) (*ArcTast, error) {
	a := s.PreValue().(arc.PreData).ARC
	cr := s.PreValue().(arc.PreData).Chrome
	device, err := ui.NewDevice(ctx, a)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create Test API connection")
	}

	return &ArcTast{arc: a, cr: cr, device: device, tconn: tconn}, err
}

//Close releases resources associated with t.
func (t *ArcTast) Close() {
	defer t.device.Close()
}

// InstallAPK prepares the android app for a test.
func (t *ArcTast) InstallAPK(ctx context.Context, path string) (err error) {
	if err := t.arc.Install(ctx, path); err != nil {
		return err
	}
	return
}

// StartActivity starts the testing activity in app.
func (t *ArcTast) StartActivity(ctx context.Context, param TestParameters) (act *arc.Activity, err error) {

	if param.Permission != "" {
		if err := t.arc.Command(ctx, "pm", "grant", pkg, param.Permission).Run(testexec.DumpLogOnError); err != nil {
			return nil, errors.Wrap(err, "failed to grant permission")
		}
	}

	act, err = arc.NewActivity(t.arc, pkg, param.Class)
	if err != nil {
		return act, errors.Wrap(err, "failed to create activity")
	}

	if err := act.Start(ctx, t.tconn); err != nil {
		return act, errors.Wrap(err, "failed to start activity")
	}
	return
}

// Verify01Result verifies the test results. The test results can be either '0' or '1' on the test App UI, where '0' means fail and '1'
// means pass.
func (t *ArcTast) Verify01Result(ctx context.Context) (ok bool, reason string, err error) {
	if err := t.device.Object(ui.ID(resultID), ui.TextMatches("[01]")).WaitForExists(ctx, 20*time.Second); err != nil {
		return false, "", errors.Wrap(err, "timed out for waiting result updated")
	}

	if result, err := t.device.Object(ui.ID(resultID)).GetText(ctx); err != nil {
		return false, "", errors.Wrap(err, "failed to get the result")
	} else if result != "1" {
		// Note: failure reason reported from the app is one line,
		// so directly print it here.
		reason, err := t.device.Object(ui.ID(logID)).GetText(ctx)
		if err != nil {
			return false, "", errors.Wrap(err, "failed to get failure reason")
		}
		return false, reason, nil
	}
	return true, "", nil
}
