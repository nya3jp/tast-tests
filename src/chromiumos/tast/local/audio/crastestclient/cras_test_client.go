// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package crastestclient provides functions to interact cras_test_client
package crastestclient

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/testing"
)

type cmdMode int

const (
	captureMode cmdMode = iota
	playbackMode
)

// blockSize calculates default block size from rate. This should be aligned as defined in cras_test_client.
func blockSize(rate int) int {
	const playbackBufferedTimeUs = 5000
	return rate * playbackBufferedTimeUs / 1000000
}

// crasTestClientCommand creates a cras_test_client command.
func crasTestClientCommand(ctx context.Context, mode cmdMode, file string, duration, channels, blocksize, rate int) *testexec.Cmd {
	runStr := "--playback_file"
	if mode == captureMode {
		runStr = "--capture_file"
	}

	return testexec.CommandContext(
		ctx, "cras_test_client",
		runStr, file,
		"--duration", strconv.Itoa(duration),
		"--num_channels", strconv.Itoa(channels),
		"--block_size", strconv.Itoa(blocksize),
		"--rate", strconv.Itoa(rate))
}

// PlaybackFileCommand creates a cras_test_client playback-from-file command.
func PlaybackFileCommand(ctx context.Context, file string, duration, channels, rate int) *testexec.Cmd {
	return crasTestClientCommand(ctx, playbackMode, file, duration, channels, blockSize(rate), rate)
}

// PlaybackCommand creates a cras_test_client playback command.
func PlaybackCommand(ctx context.Context, duration, blocksize int) *testexec.Cmd {
	return crasTestClientCommand(ctx, playbackMode, "/dev/zero", duration, 2, blocksize, 48000)
}

// CaptureFileCommand creates a cras_test_client capture-to-file command.
func CaptureFileCommand(ctx context.Context, file string, duration, channels, rate int) *testexec.Cmd {
	return crasTestClientCommand(ctx, captureMode, file, duration, channels, blockSize(rate), rate)
}

// CaptureCommand creates a cras_test_client capture command.
func CaptureCommand(ctx context.Context, duration, blocksize int) *testexec.Cmd {
	return crasTestClientCommand(ctx, captureMode, "/dev/null", duration, 2, blocksize, 48000)
}

// FirstRunningDevice returns the first input/output device by parsing audio thread logs.
// A device may not be opened immediately so it will repeat a query until there is a running device or timeout.
func FirstRunningDevice(ctx context.Context, streamType audio.StreamType) (string, error) {
	var re *regexp.Regexp
	if streamType == audio.InputStream {
		re = regexp.MustCompile("Input dev: (.*)")
	} else if streamType == audio.OutputStream {
		re = regexp.MustCompile("Output dev: (.*)")
	} else {
		return "", errors.Errorf("unsupported StreamType %d", streamType)
	}

	var devName string
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		testing.ContextLog(ctx, "Dump audio thread to check running devices")
		dump, err := testexec.CommandContext(ctx, "cras_test_client", "--dump_audio_thread").Output()
		if err != nil {
			return testing.PollBreak(errors.Errorf("failed to dump audio thread: %s", err))
		}

		dev := re.FindStringSubmatch(string(dump))
		if dev == nil {
			return errors.New("no such device")
		}

		devName = dev[1]
		return nil
	}, &testing.PollOptions{Timeout: 3 * time.Second}); err != nil {
		return "", err
	}

	return devName, nil
}

// StreamInfo holds attributes of an active stream.
// It contains only test needed fields.
type StreamInfo struct {
	Direction   string
	Effects     uint64
	FrameRate   uint32
	NumChannels uint8
	IsPinned    bool
	ClientType  string
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
		isPinned    = "is_pinned"
		ClientType  = "client_type"
	)

	// Checks all key exists.
	for _, k := range []string{Direction, Effects, FrameRate, NumChannels, isPinned, ClientType} {
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

	pinned, err := strconv.ParseUint(res[isPinned], 10, 32)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse StreamInfo::%s (value: %s)", isPinned, res[isPinned])
	}

	return &StreamInfo{
		Direction:   res[Direction],
		Effects:     effects,
		FrameRate:   uint32(frameRate),
		NumChannels: uint8(numChannels),
		IsPinned:    pinned != 0,
		ClientType:  res[ClientType],
	}, nil
}

// PollStreamResult is the CRAS stream polling result.
type PollStreamResult struct {
	Streams []StreamInfo
	Error   error
}

// StartPollStreamWorker starts a goroutine to poll an active stream.
func StartPollStreamWorker(ctx context.Context, timeout time.Duration) <-chan PollStreamResult {
	resCh := make(chan PollStreamResult, 1)
	go func() {
		streams, err := WaitForStreams(ctx, timeout)
		resCh <- PollStreamResult{Streams: streams, Error: err}
	}()
	return resCh
}

// WaitForStreams returns error if it fails to detect any active streams.
func WaitForStreams(ctx context.Context, timeout time.Duration) ([]StreamInfo, error) {
	var streams []StreamInfo

	err := testing.Poll(ctx, func(ctx context.Context) error {
		testing.ContextLog(ctx, "Polling active stream")
		var err error
		streams, err = dumpActiveStreams(ctx)
		if err != nil {
			return testing.PollBreak(errors.Errorf("failed to parse audio dumps: %s", err))
		}
		if len(streams) == 0 {
			return &NoStreamError{E: errors.New("no stream detected")}
		}
		// There is some active streams.
		return nil
	}, &testing.PollOptions{Timeout: timeout})
	return streams, err
}

// WaitForNoStream returns error if it fails to wait for all active streams to stop.
func WaitForNoStream(ctx context.Context, timeout time.Duration) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		testing.ContextLog(ctx, "Wait until there is no active stream")
		streams, err := dumpActiveStreams(ctx)
		if err != nil {
			return testing.PollBreak(errors.Errorf("failed to parse audio dumps: %s", err))
		}
		if len(streams) > 0 {
			return errors.New("active stream detected")
		}
		// No active stream.
		return nil
	}, &testing.PollOptions{Timeout: timeout})
}

// NoStreamError represents the error of failing to detect any active streams.
type NoStreamError struct {
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
func dumpActiveStreams(ctx context.Context) ([]StreamInfo, error) {
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
