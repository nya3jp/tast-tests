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
	// Apk holds is the testing App.
	Apk = "ArcAudioTest.apk"
	pkg = "org.chromium.arc.testapp.arcaudiotestapp"

	// UI IDs in the app.
	idPrefix              = pkg + ":id/"
	resultID              = idPrefix + "test_result"
	logID                 = idPrefix + "test_result_log"
	untilNoStreamsTimeout = 10 * time.Second
)

// ArcAudioTast holds the resource that needed by ARC audio tast test steps.
type ArcAudioTast struct {
	arc   *arc.ARC
	cr    *chrome.Chrome
	tconn *chrome.TestConn
}

// KeyValue is a map of string type key and string type value.
type KeyValue map[string]string

func untilNoStreams(ctx context.Context) error {
	testing.ContextLog(ctx, "Wait until there is no active stream")
	streams, _, err := DumpActiveStreams(ctx)
	if err != nil {
		return testing.PollBreak(errors.Errorf("failed to parse audio dumps: %s", err))
	}

	if len(streams) > 0 {
		return errors.New("active stream detected")
	}

	// No active stream.
	return nil

}

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

// RunAppTest runs the test that result can be either '0' or '1' on the test App UI, where '0' means fail and '1'
func RunAppTest(ctx context.Context, a *arc.ARC, cr *chrome.Chrome, apkPath string, param TestParameters) error {
	atast, err := NewArcAudioTast(ctx, a, cr)
	if err != nil {
		return errors.Wrap(err, "failed to init test case")
	}
	testing.ContextLog(ctx, "Installing app")
	if err := atast.installAPK(ctx, apkPath); err != nil {
		return errors.Wrap(err, "failed to install app")
	}
	testing.ContextLog(ctx, "Starting test activity")

	act, err := atast.startActivity(ctx, param)
	if err != nil {
		return errors.Wrap(err, "failed to start activity")
	}
	defer act.Close()
	testing.ContextLog(ctx, "Verifying App UI result")
	return atast.verifyAppResult(ctx)
}

// RunAppAndPollingTest runs the test that verifies the '0' or '1' result on the test App UI, where '0' means fail and '1'
// means pass and also starts a goroutine to verifies the polling result as well. Both the App result and polling results need to be passed.
func RunAppAndPollingTest(ctx context.Context, a *arc.ARC, cr *chrome.Chrome, apkPath string, param TestParameters, f func(context.Context) error, timeout time.Duration) error {
	atast, err := NewArcAudioTast(ctx, a, cr)
	if err != nil {
		return errors.Wrap(err, "failed to init test case")
	}
	testing.ContextLog(ctx, "Installing app")
	if err := atast.installAPK(ctx, apkPath); err != nil {
		return errors.Wrap(err, "failed to install app")
	}

	// There is an empty output stream opened after ARC booted, and we want to start the test until that stream is closed.
	if err := testing.Poll(ctx, untilNoStreams, &testing.PollOptions{Timeout: untilNoStreamsTimeout}); err != nil {
		return errors.Wrap(err, "untilNoStreams failed")
	}

	// Starts a goroutine to verifies the polling result.
	// The pollFunc can fail the test case by s.Error()
	var wg sync.WaitGroup
	wg.Add(1)

	//Inits the polling error to nil.
	var pollerr error
	go func() {
		defer wg.Done()
		if err := testing.Poll(ctx, f, &testing.PollOptions{Timeout: timeout}); err != nil {
			pollerr = errors.Wrap(err, "polling result failed")
		}
	}()

	testing.ContextLog(ctx, "Starting test activity")
	act, err := atast.startActivity(ctx, param)
	if err != nil {
		return errors.Wrap(err, "failed to start activity")
	}
	defer act.Close()
	// Waits until goroutine finish verifying poll result.
	wg.Wait()
	if pollerr != nil {
		return pollerr
	}
	testing.ContextLog(ctx, "Verifying app UI result")
	return atast.verifyAppResult(ctx)
}

// NewArcAudioTast creates an `ArcAudioTast`.
func NewArcAudioTast(ctx context.Context, a *arc.ARC, cr *chrome.Chrome) (*ArcAudioTast, error) {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create Test API connection")
	}

	return &ArcAudioTast{arc: a, cr: cr, tconn: tconn}, nil
}

func (t *ArcAudioTast) installAPK(ctx context.Context, path string) error {
	return t.arc.Install(ctx, path)
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

	if err = act.Start(ctx, t.tconn); err != nil {
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
	if err := device.Object(ui.ID(resultID), ui.TextMatches("[01]")).WaitForExists(ctx, 20*time.Second); err != nil {
		return errors.Wrap(err, "timed out for waiting result updated")
	}

	if result, err := device.Object(ui.ID(resultID)).GetText(ctx); err != nil {
		return errors.Wrap(err, "failed to get the result")
	} else if result != "1" {
		// Note: failure reason reported from the app is one line,
		// so directly print it here.
		reason, err := device.Object(ui.ID(logID)).GetText(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get failure reason")
		}
		return errors.New(reason)
	}
	return nil
}
