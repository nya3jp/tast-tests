// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package audio contains common utilities to help writing ARC audio tests.
package audio

import (
	"context"
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
	Apk = "ArcAudioTest.apk"
	pkg = "org.chromium.arc.testapp.arcaudiotestapp"

	// UI IDs in the app.
	idPrefix = pkg + ":id/"
	resultID = idPrefix + "test_result"
	logID    = idPrefix + "test_result_log"
)

// ArcAudioTast holds the resource that needed across ARC audio tast test steps.
type ArcAudioTast struct {
	arc   *arc.ARC
	cr    *chrome.Chrome
	ctx   context.Context
	tconn *chrome.TestConn
}

// RunAppTest runs the test that result can be either '0' or '1' on the test App UI, where '0' means fail and '1'
// means pass.
func (t *ArcAudioTast) RunAppTest(apkPath string, param TestParameters) error {
	testing.ContextLog(t.ctx, "Installing app")
	if err := t.installAPK(apkPath); err != nil {
		return errors.Wrap(err, "failed to install app")
	}
	testing.ContextLog(t.ctx, "Starting test activity")
	err := t.startActivity(param)
	if err != nil {
		return errors.Wrap(err, "failed to start activity")
	}
	testing.ContextLog(t.ctx, "Verifying App UI result")
	return t.verifyAppResult()
}

// NewArcAudioTast creates an ArcAudioTast.
func NewArcAudioTast(ctx context.Context, a *arc.ARC, cr *chrome.Chrome) (*ArcAudioTast, error) {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create Test API connection")
	}

	return &ArcAudioTast{arc: a, ctx: ctx, cr: cr, tconn: tconn}, nil
}

func (t *ArcAudioTast) installAPK(path string) error {
	return t.arc.Install(t.ctx, path)
}

func (t *ArcAudioTast) startActivity(param TestParameters) error {
	if param.Permission != "" {
		if err := t.arc.Command(t.ctx, "pm", "grant", pkg, param.Permission).Run(testexec.DumpLogOnError); err != nil {
			return errors.Wrap(err, "failed to grant permission")
		}
	}

	act, err := arc.NewActivity(t.arc, pkg, param.Class)
	if err != nil {
		return errors.Wrap(err, "failed to create activity")
	}
	defer act.Close()

	if err := act.Start(t.ctx, t.tconn); err != nil {
		return errors.Wrap(err, "failed to start activity")
	}
	return nil
}

func (t *ArcAudioTast) verifyAppResult() error {
	device, err := ui.NewDevice(t.ctx, t.arc)
	if err != nil {
		return errors.Wrap(err, "failed to create ui.device")
	}
	defer device.Close()
	if err := device.Object(ui.ID(resultID), ui.TextMatches("[01]")).WaitForExists(t.ctx, 20*time.Second); err != nil {
		return errors.Wrap(err, "timed out for waiting result updated")
	}
	result, err := device.Object(ui.ID(resultID)).GetText(t.ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get the result")
	}
	if result != "1" {
		// Note: failure reason reported from the app is one line,
		// so directly print it here.
		reason, err := device.Object(ui.ID(logID)).GetText(t.ctx)
		if err != nil {
			return errors.Wrap(err, "result != 1 and failed to get failure reason")
		}
		return errors.Errorf("result != 1, reason: %s", reason)
	}
	return nil
}
