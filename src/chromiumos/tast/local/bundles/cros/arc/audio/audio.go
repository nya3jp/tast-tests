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
	// Apk holds is the testing App.
	Apk = "ArcAudioTest.apk"
	pkg = "org.chromium.arc.testapp.arcaudiotestapp"

	// UI IDs in the app.
	idPrefix = pkg + ":id/"
	resultID = idPrefix + "test_result"
	logID    = idPrefix + "test_result_log"
)

// ArcAudioTast holds the resource that needed by ARC audio tast test steps.
type ArcAudioTast struct {
	arc   *arc.ARC
	cr    *chrome.Chrome
	tconn *chrome.TestConn
}

// RunAppTest runs the test that result can be either '0' or '1' on the test App UI, where '0' means fail and '1'
// means pass.
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
