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
	"chromiumos/tast/local/chrome/browser"
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
	br        *browser.Browser
	server    *MemoryStressServer
	unit      *MemoryStressUnit
	focus     *browser.Conn
	protected bool
}

// Run opens a chrome tab.
func (c *TabCanary) Run(ctx context.Context) error {
	if err := c.unit.Run(ctx, c.br); err != nil {
		return err
	}
	// Open a new tab so that the canary is not focused. Tab manager will never
	// kill the focused tab.
	var opts []browser.CreateTargetOption
	if c.protected {
		// If we want the canary to be PROTECTED_BACKGROUND priority, we want
		// to leave it visible, so make the focus capturing tab in a new window.
		opts = append(opts, browser.WithNewWindow())
	}

	focus, err := c.br.NewConn(ctx, "", opts...)
	if err != nil {
		return errors.Wrap(err, "failed to open a blank tab to ")
	}
	c.focus = focus
	return nil
}

// Close closes the chrome tab and the test server for it.
func (c *TabCanary) Close(ctx context.Context) {
	c.server.Close()
	c.unit.Close(ctx, c.br)
	c.focus.CloseTarget(ctx)
}

// StillAlive checks if the underlying chrome tab is still alive.
func (c *TabCanary) StillAlive(ctx context.Context) bool {
	return c.unit.StillAlive(ctx, c.br)
}

// String returns a string name of the canary.
func (c *TabCanary) String() string {
	return "Tab"
}

// NewTabCanary creates a new TabCanary.
// ctx       - The context the test is running on.
// allocMib  - The amount of memory allocated to the canary.
// ratio     - How compressible the allocated memory will be.
// s         - FileSystem to initilalize the memory stress server.
// br        - Browser to open the tab on.
// protected - If true, the canary will be the forground tab of a non-focused window. Otherwise, it will be a background tab in the focused window.
func NewTabCanary(ctx context.Context, allocMiB int, ratio float32, s http.FileSystem, br *browser.Browser, protected bool) Canary {
	server := NewMemoryStressServer(s)
	// Use background=true
	unit := server.NewMemoryStressUnit(allocMiB, ratio, 2*time.Second)
	return &TabCanary{br, server, unit, nil, protected}
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
