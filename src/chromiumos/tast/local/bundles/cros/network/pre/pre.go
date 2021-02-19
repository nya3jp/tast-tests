// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package pre contains preconditions for network tests.
package pre

import (
	"context"
	"time"

	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

// Result is made available to users of this precondition via:
//	func DoSomething(ctx context.Context, s *testing.State) {
//		d := s.PreValue().(pre.Result)
//		...
//	}
type Result struct {
	LogConfig
}

// preImpl implements testing.Precondition.
type preImpl struct {
	name    string        // testing.Precondition.String
	timeout time.Duration // testing.Precondition.Timeout
	logging Logging
}

// Prepare initializes the shared logging level and tags.
func (p *preImpl) Prepare(ctx context.Context, s *testing.PreState) interface{} {
	ctx, st := timing.Start(ctx, p.name+"_prepare")
	defer st.End()
	logConfig, err := p.logging.Start(ctx)
	if err != nil {
		s.Fatal("Failed to start logging: ", err)
	}
	s.Logf("Original logging level: %d, tags: %v", p.logging.OriginalLogLevel, p.logging.OriginalLogTags)
	s.Logf("Configuring logging level: %d, tags: %v", logConfig.Level, logConfig.Tags)
	return Result{LogConfig: *logConfig}
}

// Close sets the logging level to 0 and remove all the tags.
func (p *preImpl) Close(ctx context.Context, s *testing.PreState) {
	ctx, st := timing.Start(ctx, p.name+"_close")
	defer st.End()
	if err := p.logging.Stop(ctx); err != nil {
		s.Fatal("Failed to stop logging: ", err)
	}
}

func (p *preImpl) String() string { return p.name }

func (p *preImpl) Timeout() time.Duration { return p.timeout }

// NewPrecondition creates a new precondition that can be shared by tests.
func NewPrecondition(name, tags string) testing.Precondition {
	return &preImpl{
		name:    name + "_" + tags,
		timeout: 30 * time.Second,
		logging: Logging{ExtraTags: tags},
	}
}

var setLoggingWiFiPre = NewPrecondition("set_logging", "wifi")

// SetLoggingWiFi returns a precondition that WiFi logging is setup when a test is run.
func SetLoggingWiFi() testing.Precondition { return setLoggingWiFiPre }

var setLoggingCellularPre = NewPrecondition("set_logging", "cellular")

// SetLoggingCellular returns a precondition that Cellular logging is setup when a test is run.
func SetLoggingCellular() testing.Precondition { return setLoggingCellularPre }
