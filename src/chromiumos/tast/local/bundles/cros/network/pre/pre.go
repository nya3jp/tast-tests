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
	defaultLogLevel = -4
	// Default log tags used in all connectivity tests.
	// Use string instead of []string as slice cannot be const.
	// https://golang.org/doc/effective_go.html#constants
	defaultLogTags = "connection+dbus+device+link+manager+portal+service"
)

// The LogConfig object is made available to users of this precondition via:
//
//	func DoSomething(ctx context.Context, s *testing.State) {
//		d := s.PreValue().(pre.LogConfig)
//		...
//	}
type LogConfig struct {
	Level int
	Tags  string
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

	p.originalLogLevel, err = manager.GetDebugLevel(ctx)
	if err != nil {
		s.Fatal("Failed to get the debug level: ", err)
	}
	p.originalLogTags, err = manager.GetDebugTags(ctx)
	if err != nil {
		s.Fatal("Failed to get the debug tags: ", err)
	}
	s.Logf("Original logging level: %d, tags: %v", p.originalLogLevel, p.originalLogTags)

	if err := manager.SetDebugLevel(ctx, defaultLogLevel); err != nil {
		s.Fatal("Failed to set the debug level: ", err)
	}

	tags := append(strings.Split(defaultLogTags, "+"), strings.Split(p.extraTags, "+")...)
	if err := manager.SetDebugTags(ctx, tags); err != nil {
		s.Fatal("Failed to set the debug tags: ", err)
	}
	s.Logf("Configuring logging level: %d, tags: %v", defaultLogLevel, tags)

	return LogConfig{defaultLogLevel, strings.Join([]string{defaultLogTags, p.extraTags}, "+")}
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

	if err := manager.SetDebugLevel(ctx, p.originalLogLevel); err != nil {
		s.Fatal("Failed to set the debug level: ", err)
	}

	if err := manager.SetDebugTags(ctx, p.originalLogTags); err != nil {
		s.Fatal("Failed to set the debug tags: ", err)
	}
}

// NewPrecondition creates a new precondition that can be shared by tests.
func NewPrecondition(name, tags string) testing.Precondition {
	return &preImpl{
		name:      name + "_" + tags,
		timeout:   30 * time.Second,
		extraTags: tags,
	}
}

var setLoggingWiFiPre = NewPrecondition("set_logging", "wifi")

// SetLoggingWiFi returns a precondition that WiFi logging is setup when a test is run.
func SetLoggingWiFi() testing.Precondition { return setLoggingWiFiPre }

// preImpl implements testing.Precondition.
type preImpl struct {
	name             string        // testing.Precondition.String
	timeout          time.Duration // testing.Precondition.Timeout
	extraTags        string        // the tags that need to be logged such as "wifi+vpn"
	originalLogLevel int
	originalLogTags  []string
}

func (p *preImpl) String() string         { return p.name }
func (p *preImpl) Timeout() time.Duration { return p.timeout }
