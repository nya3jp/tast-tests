// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package pre

import (
	"context"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/shill"
)

const (
	defaultLogLevel = -4
	// Default log tags used in all connectivity tests.
	// Use string instead of []string as slice cannot be const.
	// https://golang.org/doc/effective_go.html#constants
	defaultLogTags = "connection+dbus+device+link+manager+portal+service"
)

// The LogConfig object is provided as part of pre.Result.
type LogConfig struct {
	Level int
	Tags  string
}

// Logging implements logging preconditions for network tests.
type Logging struct {
	ExtraTags        string // the tags that need to be logged such as "wifi+vpn"
	OriginalLogLevel int
	OriginalLogTags  []string
}

// Start initializes the shared logging level and tags.
func (l *Logging) Start(ctx context.Context) (*LogConfig, error) {
	// Configuire the new shared logging setup.
	manager, err := shill.NewManager(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create shill manager proxy")
	}

	l.OriginalLogLevel, err = manager.GetDebugLevel(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the debug level")
	}
	l.OriginalLogTags, err = manager.GetDebugTags(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the debug tags")
	}

	if err := manager.SetDebugLevel(ctx, defaultLogLevel); err != nil {
		return nil, errors.Wrap(err, "failed to set the debug level")
	}

	tags := append(strings.Split(defaultLogTags, "+"), strings.Split(l.ExtraTags, "+")...)
	if err := manager.SetDebugTags(ctx, tags); err != nil {
		return nil, errors.Wrap(err, "failed to set the debug tags: ")
	}

	return &LogConfig{defaultLogLevel, strings.Join([]string{defaultLogTags, l.ExtraTags}, "+")}, nil
}

// Stop sets the logging level to 0 and remove all the tags.
func (l *Logging) Stop(ctx context.Context) error {
	// Restore initial logging setup.
	manager, err := shill.NewManager(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create shill manager proxy")
	}

	if err := manager.SetDebugLevel(ctx, l.OriginalLogLevel); err != nil {
		return errors.Wrap(err, "failed to set the debug level")
	}

	if err := manager.SetDebugTags(ctx, l.OriginalLogTags); err != nil {
		return errors.Wrap(err, "failed to set the debug tags ")
	}
	return nil
}
