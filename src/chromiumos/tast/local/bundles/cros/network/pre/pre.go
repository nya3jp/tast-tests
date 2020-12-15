// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package pre contains preconsitions for network tests.
package pre

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

const (
	// Default log level.
	logLevel = -4
	// Default log tags used in all connectivity tests.
	logTags = "connection+dbus+device+link+manager+portal+service"
)

// The logConfig object is made available to users of this precondition via:
//
//	func DoSomething(ctx context.Context, s *testing.State) {
//		d := s.PreValue().(pre.logConfig)
//		...
//	}
type logConfig struct {
	level int
	tags  string
}

// Prepare initializes the shared logging level and tags.
func (p *preImpl) Prepare(ctx context.Context, s *testing.PreState) interface{} {
	ctx, st := timing.Start(ctx, p.name+"_prepare")
	defer st.End()
	// configuire the new shared logging setup.
	manager, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed to create shill manager proxy: ", err)
	}

	if err := manager.SetDebugLevel(ctx, logLevel); err != nil {
		s.Fatal("Failed to set the debug level: ", err)
	}

	tags := append(strings.Split(logTags, "+"), strings.Split(p.extraTags, "+")...)
	if err := manager.SetDebugTags(ctx, tags); err != nil {
		s.Fatal("Failed to set the debug tags: ", err)
	}
	s.Logf("Configured logging level = %d, tags = %v", logLevel, tags)

	return logConfig{logLevel, logTags + p.extraTags}
}

// Close sets the logging level to 0 and remove all the tags.
func (p *preImpl) Close(ctx context.Context, s *testing.PreState) {
	ctx, st := timing.Start(ctx, p.name+"_close")
	defer st.End()
	// Restore initial logging setup.
	manager, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed to create shill manager proxy: ", err)
	}

	if err := manager.SetDebugLevel(ctx, 0); err != nil {
		s.Fatal("Failed to set the debug level: ", err)
	}

	if err := manager.SetDebugTags(ctx, []string{}); err != nil {
		s.Fatal("Failed to set the debug tags: ", err)
	}
}

// SetLoggingWiFi returns a precondition that WiFi logging is setup when a test is run.
func SetLoggingWiFi() testing.Precondition { return setLoggingWiFiPre }

// NewPrecondition creates a new precondition that can be shared by tests.
func NewPrecondition(name, tags string) testing.Precondition {
	return &preImpl{
		name:      name + "_" + tags,
		timeout:   30 * time.Second,
		extraTags: tags,
	}
}

var setLoggingWiFiPre = NewPrecondition("set_logging", "wifi")

// preImpl implements testing.Precondition.
type preImpl struct {
	name      string        // testing.Precondition.String
	timeout   time.Duration // testing.Precondition.Timeout
	extraTags string        // the tags that need to be logged such as "wifi+vpn"
}

func (p *preImpl) String() string         { return p.name }
func (p *preImpl) Timeout() time.Duration { return p.timeout }
