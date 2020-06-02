// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package audio contains common utilities to help writing ARC audio tests.
package audio

import (
	"context"
	"regexp"
	"strconv"
	"strings"
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

// ARCAudioTast holds the resource that needed across ARC audio tast test steps.
type ARCAudioTast struct {
	arc   *arc.ARC
	cr    *chrome.Chrome
	tconn *chrome.TestConn
}

// StreamInfo holds attributes of an active stream.
// It contains only test needed fields.
type StreamInfo struct {
	Direction   string
	Effects     uint64
	FrameRate   uint32
	NumChannels uint8
}

var streamInfoRegex = regexp.MustCompile("(.*):(.*)")

func newStreamInfo(s string) (*StreamInfo, error) {
	data := streamInfoRegex.FindAllStringSubmatch(s, -1)
	res := make(map[string]string)
	for _, kv := range data {
		k := kv[1]
		v := strings.Trim(kv[2], " ")
		res[k] = v
	}

	const (
		Direction   = "direction"
		Effects     = "effects"
		FrameRate   = "frame_rate"
		NumChannels = "num_channels"
	)

	// Checks all key exists.
	for _, k := range []string{Direction, Effects, FrameRate, NumChannels} {
		if _, ok := res[k]; !ok {
			return nil, errors.Errorf("missing key: %s in StreamInfo", k)
		}
	}

	effects, err := strconv.ParseUint(res[Effects], 0, 64)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse StreamInfo::%s (value: %s)", Effects, res[Effects])
	}

	frameRate, err := strconv.ParseUint(res[FrameRate], 10, 32)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse StreamInfo::%s (value: %s)", FrameRate, res[FrameRate])
	}
	numChannels, err := strconv.ParseUint(res[NumChannels], 10, 8)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse StreamInfo::%s (value: %s)", NumChannels, res[NumChannels])
	}

	return &StreamInfo{
		Direction:   res[Direction],
		Effects:     effects,
		FrameRate:   uint32(frameRate),
		NumChannels: uint8(numChannels),
	}, nil
}

// waitForStreams returns error if it fails to detect any active streams.
func (t *ARCAudioTast) waitForStreams(ctx context.Context) ([]StreamInfo, error) {
	var streams []StreamInfo

	err := testing.Poll(ctx, func(ctx context.Context) error {
		testing.ContextLog(ctx, "Polling active stream")
		var err error
		streams, err = t.dumpActiveStreams(ctx)
		if err != nil {
			return testing.PollBreak(errors.Errorf("failed to parse audio dumps: %s", err))
		}
		if len(streams) == 0 {
			return &noStreamError{E: errors.New("no stream detected")}
		}
		// There is some active streams.
		return nil
	}, &testing.PollOptions{Timeout: hasStreamsTimeout})
	return streams, err
}

// waitForNoStream returns error if it fails to wait for all active streams to stop.
func (t *ARCAudioTast) waitForNoStream(ctx context.Context) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		testing.ContextLog(ctx, "Wait until there is no active stream")
		streams, err := t.dumpActiveStreams(ctx)
		if err != nil {
			return testing.PollBreak(errors.Errorf("failed to parse audio dumps: %s", err))
		}
		if len(streams) > 0 {
			return errors.New("active stream detected")
		}
		// No active stream.
		return nil
	}, &testing.PollOptions{Timeout: noStreamsTimeout})
}

type noStreamError struct {
	*errors.E
}

// dumpActiveStreams parses active streams from "cras_test_client --dump_audio_thread" log.
// The log format is defined in cras_test_client.c.
// The active streams section begins with: "-------------stream_dump------------" and ends with: "Audio Thread Event Log:"
// Each stream is separated by "\n\n"
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
// stream: 94437379 dev: 2
// ...
//
// Audio Thread Event Log:
//
func (t *ARCAudioTast) dumpActiveStreams(ctx context.Context) ([]StreamInfo, error) {
	dump, err := testexec.CommandContext(ctx, "cras_test_client", "--dump_audio_thread").Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Errorf("failed to dump audio thread: %s", err)
	}

	streamSection := strings.Split(string(dump), "-------------stream_dump------------")
	if len(streamSection) != 2 {
		return nil, errors.New("failed to split log by stream_dump")
	}
	streamSection = strings.Split(streamSection[1], "Audio Thread Event Log:")
	if len(streamSection) == 1 {
		return nil, errors.New("invalid stream_dump")
	}
	str := strings.Trim(streamSection[0], " \n\t")

	// No active streams, return nil
	if str == "" {
		return nil, nil
	}

	var streams []StreamInfo
	for _, streamStr := range strings.Split(str, "\n\n") {
		stream, err := newStreamInfo(streamStr)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse stream")
		}
		streams = append(streams, *stream)
	}
	return streams, nil
}

// RunAppTest runs the test that result can be either '0' or '1' on the test App UI, where '0' means fail and '1'
// means pass.
func (t *ARCAudioTast) RunAppTest(ctx context.Context, apkPath string, param TestParameters) error {
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

type pollStreamResult struct {
	Streams []StreamInfo
	Error   error
}

func (t *ARCAudioTast) startPollStreamWorker(ctx context.Context) <-chan pollStreamResult {
	resCh := make(chan pollStreamResult, 1)
	go func() {
		streams, err := t.waitForStreams(ctx)
		resCh <- pollStreamResult{Streams: streams, Error: err}
	}()
	return resCh
}

// RunAppAndPollStream verifies the '0' or '1' result on the test App UI, where '0' means fail and '1'
// means pass and it also starts a goroutine to poll the audio streams created by the test App.
func (t *ARCAudioTast) RunAppAndPollStream(ctx context.Context, apkPath string, param TestParameters) ([]StreamInfo, error) {

	testing.ContextLog(ctx, "Installing app")
	if err := t.arc.Install(ctx, apkPath); err != nil {
		return nil, errors.Wrap(err, "failed to install app")
	}
	// There is an empty output stream opened after ARC booted, and we want to start the test until that stream is closed.
	if err := t.waitForNoStream(ctx); err != nil {
		return nil, errors.Wrap(err, "timeout waiting all stream stopped")
	}

	// Starts a goroutine to poll the audio streams created by the test App.
	resCh := t.startPollStreamWorker(ctx)

	testing.ContextLog(ctx, "Starting test activity")
	act, err := t.startActivity(ctx, param)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start activity")
	}
	defer act.Close()

	// verifying poll stream result.
	res := <-resCh
	if res.Error != nil {
		// Returns error, if it is not a noStreamError
		var e *noStreamError
		if !errors.As(res.Error, &e) {
			return nil, res.Error
		}
	}

	testing.ContextLog(ctx, "Verifying app UI result")
	if err := t.verifyAppResult(ctx); err != nil {
		return nil, err
	}
	return res.Streams, nil
}

// NewARCAudioTast creates an ARCAudioTast.
func NewARCAudioTast(ctx context.Context, a *arc.ARC, cr *chrome.Chrome) (*ARCAudioTast, error) {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create Test API connection")
	}

	return &ARCAudioTast{arc: a, cr: cr, tconn: tconn}, nil
}

func (t *ARCAudioTast) startActivity(ctx context.Context, param TestParameters) (*arc.Activity, error) {
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

func (t *ARCAudioTast) verifyAppResult(ctx context.Context) error {
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
