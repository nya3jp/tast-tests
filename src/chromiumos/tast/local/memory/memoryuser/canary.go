// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package memoryuser

import (
	"context"
	"net/http"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
)

// CanaryType is an enum listing up the available canaries of
// MemoryAllocationCanaryHealthPerf.
type CanaryType int64

const (
	// Tab is for a Chrome tab canary.
	Tab CanaryType = iota
	// App is for an Arc testing app canary.
	App
)

// Canary describes available operations for canaries
// listed up in CanaryType.
type Canary interface {
	Run(ctx context.Context) error
	Close(ctx context.Context)
	StillAlive(ctx context.Context) bool
	String() string
}

// TabCanary is a canary implemented with a chrome tab.
type TabCanary struct {
	cr     *chrome.Chrome
	server *MemoryStressServer
	unit   *MemoryStressUnit
}

// AppCanary is a canary implemented with an ARC test app.
type AppCanary struct {
	a     *arc.ARC
	tconn *chrome.TestConn
	unit  *ArcLifecycleUnit
}

// Run opens a chrome tab.
func (c *TabCanary) Run(ctx context.Context) error {
	return c.unit.Run(ctx, c.cr)
}

// Close closes the chrome tab and the test server for it.
func (c *TabCanary) Close(ctx context.Context) {
	c.server.Close()
	c.unit.Close(ctx, c.cr)
}

// StillAlive checks if the underlying chrome tab is still alive.
func (c *TabCanary) StillAlive(ctx context.Context) bool {
	return c.unit.StillAlive(ctx, c.cr)
}

// String returns a string name of the canary.
func (c *TabCanary) String() string {
	return "Tab"
}

// Run opens a test ARC app.
func (c *AppCanary) Run(ctx context.Context) error {
	return c.unit.Run(ctx, c.a, c.tconn)
}

// Close closes the test app.
func (c *AppCanary) Close(ctx context.Context) {
	c.unit.Close(ctx, c.a)
}

// StillAlive checks if the app is stilol alive.
func (c *AppCanary) StillAlive(ctx context.Context) bool {
	return c.unit.StillAlive(ctx, c.a)
}

// String returns a string name of the canary.
func (c *AppCanary) String() string {
	return "App"
}

// NewCanary creates a new Canary.
// ctx 		- The context the test is running on.
// t   		- The type of canary you want to use.
// allocMib - The amount of memory allocated to the canary.
// ratio	- How compressible the allocated memory will be.
// s		- FileSystem to initilalize the memory stress server.
// cr		- Chrome to open the tab on. It can be null if the canary does not use a tab.
// a		- ARC to run the test app on. It can be null if the cnaary does not use an app.
func NewCanary(ctx context.Context, t CanaryType, allocMiB int, ratio float32, s http.FileSystem, cr *chrome.Chrome, a *arc.ARC) (Canary, error) {
	switch t {
	case Tab:
		server := NewMemoryStressServer(s)
		// Use background=true
		unit := server.NewMemoryStressUnit(50, 0.67, 2*time.Second, true)
		return &TabCanary{cr, server, unit}, nil
	case App:
		if err := InstallArcLifecycleTestApps(ctx, a, 1); err != nil {
			return nil, errors.Wrap(err, "failed to install the test app")
		}
		unit := NewArcLifecycleUnit(0, 50, 0.67, nil, true)
		tconn, err := cr.TestAPIConn(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create a test API connection")
		}
		return &AppCanary{a, tconn, unit}, nil
	default:
		//Never reaches
		return nil, nil
	}
}
