// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package testutil provides utilities to setup testing environment for camera
// tests.
package testutil

import (
	"context"
	"time"

	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

var preconditions = make(map[string]testing.Precondition)

// ChromeConfig is the config to set the desired features/switches when
// launching Chrome.
type ChromeConfig struct {
	InstallSWA              bool
	UseFakeCamera           bool
	UseFakeHumanFaceContent bool
	UseFakeDMS              bool
	ARCEnabled              bool
}

// NewPrecondition returns a precondition based on given Chrome |config|.
func NewPrecondition(config ChromeConfig) testing.Precondition {
	name := "cca"
	if config.InstallSWA {
		name += "-swa"
	}
	if config.UseFakeCamera {
		name += "-fakeCam"
	}
	if config.UseFakeHumanFaceContent {
		name += "-fakeFace"
	}
	if config.UseFakeDMS {
		name += "-fakePolicy"
	}
	if config.ARCEnabled {
		name += "-arc"
	}

	if precondition, exist := preconditions[name]; exist {
		return precondition
	}

	preconditions[name] = &preImpl{
		name:   name,
		config: config,
	}
	return preconditions[name]
}

// PreData exposes the information about the test environment.
type PreData struct {
	// Config is the config to configure Chrome instance used in current test.
	Config ChromeConfig
	// Chrome is the Chrome instance which is launched according to |Config|.
	Chrome *chrome.Chrome
	// ARC is the ARC instance if ARC is enabled in |Config|.
	ARC *arc.ARC
	// FakeDMS is used to fake policy for Chrome.
	FakeDMS *fakedms.FakeDMS
	// TestBridge is used to communicate with app for test specific logic.
	TestBridge *TestBridge
}

// preImpl implements testing.Precondition.
type preImpl struct {
	// name is the name of the precondition.
	name   string
	config ChromeConfig

	cr  *chrome.Chrome
	arc *arc.ARC

	fakeDMS    *fakedms.FakeDMS
	testBridge *TestBridge
}

func (p *preImpl) String() string         { return p.name }
func (p *preImpl) Timeout() time.Duration { return 90 * time.Second }

// Prepare setups the test environment for current precondition.
func (p *preImpl) Prepare(ctx context.Context, s *testing.PreState) interface{} {
	ctx, st := timing.Start(ctx, "prepare_"+p.name)
	defer st.End()

	if p.cr != nil {
		err := func() error {
			ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
			defer cancel()
			ctx, st := timing.Start(ctx, "reset_"+p.name)
			defer st.End()
			if err := p.cr.Responded(ctx); err != nil {
				return errors.Wrap(err, "existing Chrome connection is unusable")
			}
			if err := p.cr.ResetState(ctx); err != nil {
				return errors.Wrap(err, "failed resetting existing Chrome session")
			}
			if err := p.testBridge.Reset(ctx); err != nil {
				return errors.Wrap(err, "failed to reset test bridge")
			}
			return nil
		}()
		if err == nil {
			s.Log("Reusing existing test environment")
			return PreData{p.config, p.cr, p.arc, p.fakeDMS, p.testBridge}
		}
		chrome.Unlock()
		if err := p.cr.Close(ctx); err != nil {
			s.Log("Failed to close Chrome: ", err)
		}
	}

	opts := []chrome.Option{chrome.ExtraArgs("--camera-app-test")}
	if p.config.InstallSWA {
		opts = append(opts, chrome.ExtraArgs("--enable-features=CameraSystemWebApp"))
	}
	if p.config.UseFakeCamera {
		opts = append(opts, chrome.ExtraArgs(
			"--use-fake-ui-for-media-stream",
			// The default fps of fake device is 20, but CCA requires fps >= 24.
			// Set the fps to 30 to avoid OverconstrainedError.
			"--use-fake-device-for-media-stream=fps=30"))
	}
	if p.config.UseFakeHumanFaceContent {
		opts = append(opts, chrome.ExtraArgs(
			"--use-file-for-fake-video-capture="+s.DataPath("human_face.y4m")))
	}
	if p.config.ARCEnabled {
		opts = append(opts, chrome.ARCEnabled())
	}
	var fdms *fakedms.FakeDMS
	var err error
	if p.config.UseFakeDMS {
		fdms, err = fakedms.New(s.PreCtx(), s.OutDir())
		if err != nil {
			s.Fatal("Failed to start FakeDMS: ", err)
		}
		if err := fdms.WritePolicyBlob(fakedms.NewPolicyBlob()); err != nil {
			s.Fatal("Failed to write policies to FakeDMS: ", err)
		}
		opts = append(opts, chrome.Auth("tast-user@managedchrome.com", "test0000", "gaia-id"),
			chrome.DMSPolicy(fdms.URL))
	}

	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		s.Fatal("Failed to setup Chrome: ", err)
	}

	var a *arc.ARC
	if p.config.ARCEnabled {
		a, err = arc.New(ctx, s.OutDir())
		if err != nil {
			s.Fatal("Failed to start ARC: ", err)
		}
	}

	testBridge, err := NewTestBridge(ctx, cr, p.config.InstallSWA)
	if err != nil {
		s.Fatal("Failed to construct test bridge: ", err)
	}

	chrome.Lock()

	p.cr = cr
	p.arc = a
	p.fakeDMS = fdms
	p.testBridge = testBridge
	return PreData{p.config, p.cr, p.arc, p.fakeDMS, p.testBridge}
}

// Close is called by the test framework after the last test that uses this precondition.
func (p *preImpl) Close(ctx context.Context, s *testing.PreState) {
	ctx, st := timing.Start(ctx, "close_"+p.name)
	defer st.End()

	chrome.Unlock()

	if err := p.testBridge.TearDown(ctx); err != nil {
		s.Log("Failed to tear down test bridge: ", err)
	}
	p.testBridge = nil

	if p.arc != nil {
		p.arc.Close()
		p.arc = nil
	}

	if p.fakeDMS != nil {
		p.fakeDMS.Stop(ctx)
		p.fakeDMS = nil
	}

	if err := p.cr.Close(ctx); err != nil {
		s.Log("Failed to close Chrome: ", err)
	}
	p.cr = nil
}
