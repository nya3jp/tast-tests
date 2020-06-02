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

// TestParameters holds the ARC audio tast parameters.
type TestParameters struct {
	Permission string
	Class      string
}

const (
	// Apk is the testing App.
	Apk = "ARCAudioTest.apk"
	pkg = "org.chromium.arc.testapp.arcaudiotestapp"

	// UI IDs in the app.
	idPrefix              = pkg + ":id/"
	resultID              = idPrefix + "test_result"
	logID                 = idPrefix + "test_result_log"
	verifyUIResultTimeout = 20 * time.Second
	noStreamsTimeout      = 20 * time.Second
	hasStreamsTimeout     = 10 * time.Second
)

// ArcAudioTast holds the resource that needed across ARC audio tast test steps.
type ArcAudioTast struct {
	arc   *arc.ARC
	cr    *chrome.Chrome
	tconn *chrome.TestConn
}

// KeyValue is a map of string type key and string type value.
type KeyValue map[string]string

// noStreams is intended for use inside a poll and it returns error when it detects an active stream.
func noStreams(ctx context.Context) error {
	testing.ContextLog(ctx, "Wait until there is no active stream")
	streams, err := DumpActiveStreams(ctx)
	if err != nil {
		return testing.PollBreak(errors.Errorf("failed to parse audio dumps: %s", err))
	}
	if len(streams) > 0 {
		return errors.New("active stream detected")
	}
	// No active stream.
	return nil
}

// DumpActiveStreams parses active streams from "cras_test_client --dump_audio_thread" log.
// The active streams section begins with: "-------------stream_dump------------" and ends with: "Audio Thread Event Log:"
// An example of "cras_test_client --dump_audio_thread" log is shown as below:
// -------------stream_dump------------
// stream: 94437376 dev: 6
// direction: Output
// stream_type: CRAS_STREAM_TYPE_DEFAULT
// client_type: CRAS_CLIENT_TYPE_PCM
// buffer_frames: 2000
// cb_threshold: 1000
// effects: 0x0000
// frame_rate: 8000
// num_channels: 1
// longest_fetch_sec: 0.004927402
// num_overruns: 0
// is_pinned: 0
// pinned_dev_idx: 0
// num_missed_cb: 0
// volume: 1.000000
// runtime: 26.168175600
// channel map:0 -1 -1 -1 -1 -1 -1 -1 -1 -1 -1
//
// Audio Thread Event Log:
//
func DumpActiveStreams(ctx context.Context) ([]KeyValue, error) {
	dump, err := testexec.CommandContext(ctx, "cras_test_client", "--dump_audio_thread").Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Errorf("failed to dump audio thread: %s", err)
	}

	s := strings.Split(string(dump), "-------------stream_dump------------")
	if len(s) < 2 {
		return nil, errors.New("no stream_dump")
	}
	s = strings.Split(s[1], "Audio Thread Event Log:")
	if len(s) == 0 {
		return nil, errors.New("invalid stream_dump")
	}
	streamStr := strings.Trim(s[0], " \n\t")
	streams := make([]KeyValue, 0)

	// No active streams, return empty slice.
	if streamStr == "" {
		return streams, nil
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

	return streams, nil
}

// RunAppTest runs the test that result can be either '0' or '1' on the test App UI, where '0' means fail and '1'
// means pass.
func (t *ArcAudioTast) RunAppTest(ctx context.Context, apkPath string, param TestParameters) error {
	testing.ContextLog(ctx, "Installing app")
	if err := t.arc.Install(ctx, apkPath); err != nil {
		return errors.Wrap(err, "failed to install app")
	}
	testing.ContextLog(ctx, "Starting test activity")
	act, err := t.startActivity(ctx, param)
	if err != nil {
		return errors.Wrap(err, "failed to start activity")
	}
	defer act.Close()
	testing.ContextLog(ctx, "Verifying App UI result")
	return t.verifyAppResult(ctx)
}

// RunAppTestAndPollStream verifies the '0' or '1' result on the test App UI, where '0' means fail and '1'
// means pass and it also starts a goroutine to poll the audio streams created by the test App.
func (t *ArcAudioTast) RunAppTestAndPollStream(ctx context.Context, apkPath string, param TestParameters) ([]KeyValue, error) {
	streams := make([]KeyValue, 0)
	// This is a function intended for use inside a poll and it returns error when it fails to detect an active stream.
	hasStreams := func(ctx context.Context) error {
		testing.ContextLog(ctx, "Polling active stream")
		var err error
		streams, err = DumpActiveStreams(ctx)
		if err != nil {
			return testing.PollBreak(errors.Errorf("failed to parse audio dumps: %s", err))
		}
		if len(streams) == 0 {
			return errors.New("no stream detected")
		}
		// There is some active streams.
		return nil
	}

	testing.ContextLog(ctx, "Installing app")
	if err := t.arc.Install(ctx, apkPath); err != nil {
		return nil, errors.Wrap(err, "failed to install app")
	}
	// There is an empty output stream opened after ARC booted, and we want to start the test until that stream is closed.
	if err := testing.Poll(ctx, noStreams, &testing.PollOptions{Timeout: noStreamsTimeout}); err != nil {
		return nil, errors.Wrap(err, "timeout waiting all stream stopped")
	}

	// Starts a goroutine to poll the audio streams created by the test App.
	var wg sync.WaitGroup
	wg.Add(1)
	//Inits the polling error to nil.
	var pollerr error
	go func() {
		defer wg.Done()
		if err := testing.Poll(ctx, hasStreams, &testing.PollOptions{Timeout: hasStreamsTimeout}); err != nil {
			pollerr = errors.Wrap(err, "polling stream failed")
		}
	}()

	testing.ContextLog(ctx, "Starting test activity")
	act, err := t.startActivity(ctx, param)
	defer act.Close()
	if err != nil {
		return nil, errors.Wrap(err, "failed to start activity")
	}

	// Waits until goroutine finish verifying poll result.
	wg.Wait()
	if pollerr != nil {
		return nil, pollerr
	}
	testing.ContextLog(ctx, "Verifying app UI result")
	if err = t.verifyAppResult(ctx); err != nil {
		return nil, err
	}
	return streams, nil
}

// NewArcAudioTast creates an ArcAudioTast.
func NewArcAudioTast(ctx context.Context, a *arc.ARC, cr *chrome.Chrome) (*ArcAudioTast, error) {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create Test API connection")
	}

	return &ArcAudioTast{arc: a, cr: cr, tconn: tconn}, nil
}

func (t *ArcAudioTast) startActivity(ctx context.Context, param TestParameters) (*arc.Activity, error) {
	if param.Permission != "" {
		if err := t.arc.Command(ctx, "pm", "grant", pkg, param.Permission).Run(testexec.DumpLogOnError); err != nil {
			return nil, errors.Wrap(err, "failed to grant permission")
		}
	}

	act, err := arc.NewActivity(t.arc, pkg, param.Class)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create activity")
	}

	if err := act.Start(ctx, t.tconn); err != nil {
		return nil, errors.Wrap(err, "failed to start activity")
	}
	return act, nil
}

func (t *ArcAudioTast) verifyAppResult(ctx context.Context) error {
	device, err := ui.NewDevice(ctx, t.arc)
	if err != nil {
		return errors.Wrap(err, "failed to create ui.device")
	}
	defer device.Close()
	if err := device.Object(ui.ID(resultID), ui.TextMatches("[01]")).WaitForExists(ctx, verifyUIResultTimeout); err != nil {
		return errors.Wrap(err, "timed out for waiting result updated")
	}
	result, err := device.Object(ui.ID(resultID)).GetText(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get the result")
	}
	if result != "1" {
		// Note: failure reason reported from the app is one line,
		// so directly print it here.
		reason, err := device.Object(ui.ID(logID)).GetText(ctx)
		if err != nil {
			return errors.Wrapf(err, "failed to get failure reason for unexpected app result: got %s, want 1", result)
		}
		return errors.Errorf("unexpected app result (with reason: %s): got %s, want 1", reason, result)
	}
	return nil
}
