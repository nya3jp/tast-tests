// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package audio contains common utilities to help writing ARC audio tests.
package audio

import (
	"context"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/audio/crastestclient"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/power/setup"
	"chromiumos/tast/testing"
)

// PerformanceMode equals to AudioTrack::PERFORMANCE_MODE definition.
type PerformanceMode uint64

const (
	// PerformanceModeNone equals to AudioTrack::PERFORMANCE_MODE_NONE
	PerformanceModeNone PerformanceMode = iota
	// PerformanceModeLowLatency equals to AudioTrack::PERFORMANCE_MODE_LOW_LATENCY
	PerformanceModeLowLatency
	// PerformanceModePowerSaving equals to AudioTrack::PERFORMANCE_MODE_POWER_SAVING
	PerformanceModePowerSaving
)

// TestParameters holds the ARC audio tast parameters.
type TestParameters struct {
	Permission           string
	Class                string
	PerformanceMode      PerformanceMode
	BatteryDischargeMode setup.BatteryDischargeMode
}

const (
	// Apk is the testing App.
	Apk = "ARCAudioTest.apk"
	// Pkg is the package name the testing App.
	Pkg = "org.chromium.arc.testapp.arcaudiotest"

	// UI IDs in the app.
	idPrefix              = Pkg + ":id/"
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
	d     *ui.Device
	tconn *chrome.TestConn
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

// RunAppAndPollStream verifies the '0' or '1' result on the test App UI, where '0' means fail and '1'
// means pass and it also starts a goroutine to poll the audio streams created by the test App.
func (t *ARCAudioTast) RunAppAndPollStream(ctx context.Context, apkPath string, param TestParameters) ([]crastestclient.StreamInfo, error) {

	testing.ContextLog(ctx, "Installing app")
	if err := t.arc.Install(ctx, apkPath); err != nil {
		return nil, errors.Wrap(err, "failed to install app")
	}
	// There is an empty output stream opened after ARC booted, and we want to start the test until that stream is closed.
	if err := crastestclient.WaitForNoStream(ctx, noStreamsTimeout); err != nil {
		return nil, errors.Wrap(err, "timeout waiting all stream stopped")
	}

	// Starts a goroutine to poll the audio streams created by the test App.
	resCh := crastestclient.StartPollStreamWorker(ctx, hasStreamsTimeout)

	testing.ContextLog(ctx, "Starting test activity")
	act, err := t.startActivity(ctx, param)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start activity")
	}
	defer act.Close()

	// verifying poll stream result.
	res := <-resCh
	if res.Error != nil {
		// Returns error, if it is not a NoStreamError
		var e *crastestclient.NoStreamError
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
func NewARCAudioTast(ctx context.Context, a *arc.ARC, cr *chrome.Chrome, d *ui.Device) (*ARCAudioTast, error) {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create Test API connection")
	}

	return &ARCAudioTast{arc: a, cr: cr, d: d, tconn: tconn}, nil
}

func (t *ARCAudioTast) startActivity(ctx context.Context, param TestParameters) (*arc.Activity, error) {
	if param.Permission != "" {
		if err := t.arc.Command(ctx, "pm", "grant", Pkg, param.Permission).Run(testexec.DumpLogOnError); err != nil {
			return nil, errors.Wrap(err, "failed to grant permission")
		}
	}

	act, err := arc.NewActivity(t.arc, Pkg, param.Class)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create activity")
	}

	if err := act.Start(ctx, t.tconn); err != nil {
		return nil, errors.Wrap(err, "failed to start activity")
	}
	return act, nil
}

func (t *ARCAudioTast) verifyAppResult(ctx context.Context) error {
	if err := t.d.Object(ui.ID(resultID), ui.TextMatches("[01]")).WaitForExists(ctx, verifyUIResultTimeout); err != nil {
		return errors.Wrap(err, "timed out for waiting result updated")
	}
	result, err := t.d.Object(ui.ID(resultID)).GetText(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get the result")
	}
	if result != "1" {
		// Note: failure reason reported from the app is one line,
		// so directly print it here.
		reason, err := t.d.Object(ui.ID(logID)).GetText(ctx)
		if err != nil {
			return errors.Wrapf(err, "failed to get failure reason for unexpected app result: got %s, want 1", result)
		}
		return errors.Errorf("unexpected app result (with reason: %s): got %s, want 1", reason, result)
	}
	return nil
}
