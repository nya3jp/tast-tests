// Copyright 2022 The ChromiumOS Authors
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
type CanaryType int

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

// NewTabCanary creates a new TabCanary.
// ctx 		- The context the test is running on.
// allocMib	- The amount of memory allocated to the canary.
// ratio	- How compressible the allocated memory will be.
// s		- FileSystem to initilalize the memory stress server.
// cr		- Chrome to open the tab on.
func NewTabCanary(ctx context.Context, allocMiB int, ratio float32, s http.FileSystem, cr *chrome.Chrome) Canary {
	server := NewMemoryStressServer(s)
	// Use background=true
	unit := server.NewMemoryStressUnit(allocMiB, ratio, 2*time.Second, true)
	return &TabCanary{cr, server, unit}
}

// AppCanary is a canary implemented with an ARC test app.
type AppCanary struct {
	a     *arc.ARC
	tconn *chrome.TestConn
	unit  *ArcLifecycleUnit
}

// Run opens a test ARC app.
func (c *AppCanary) Run(ctx context.Context) error {
	return c.unit.Run(ctx, c.a, c.tconn)
}

// Close closes the test app.
func (c *AppCanary) Close(ctx context.Context) {
	c.unit.Close(ctx, c.a)
}

// StillAlive checks if the app is still alive.
func (c *AppCanary) StillAlive(ctx context.Context) bool {
	return c.unit.StillAlive(ctx, c.a)
}

// String returns a string name of the canary.
func (c *AppCanary) String() string {
	return "App"
}

// NewAppCanary creates a new Canary.
// ctx 		- The context the test is running on.
// allocMib	- The amount of memory allocated to the canary.
// ratio	- How compressible the allocated memory will be.
// cr		- Chrome to establish the test api connection.
// a		- ARC to run the test app on.
func NewAppCanary(ctx context.Context, allocMiB int, ratio float32, cr *chrome.Chrome, a *arc.ARC) (Canary, error) {
	if err := InstallArcLifecycleTestApps(ctx, a, 1); err != nil {
		return nil, errors.Wrap(err, "failed to install the test app")
	}
	unit := NewArcLifecycleUnit(0, int64(allocMiB), float64(ratio), nil, true)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create a test API connection")
	}
	return &AppCanary{a, tconn, unit}, nil
}
