// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package memoryuser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// MemoryStressUnit creates a Chrome tab that allocates memory like the
// platform.MemoryStressBasic test.
type MemoryStressUnit struct {
	url        string
	conn       *chrome.Conn
	cooldown   time.Duration
	background bool
}

// Run creates a Chrome tab that allocates memory, then waits for the provided
// cooldown.
func (st *MemoryStressUnit) Run(ctx context.Context, cr *chrome.Chrome) error {
	var conn *chrome.Conn
	var err error

	if st.background {
		conn, err = cr.NewBackgroundConn(ctx, st.url)
	} else {
		conn, err = cr.NewConn(ctx, st.url)
	}

	if err != nil {
		return errors.New("failed to open MemoryStressUnit page")
	}
	st.conn = conn

	// Wait for allocation to complete.
	const expr = "document.hasOwnProperty('out') == true"
	if err := conn.WaitForExprFailOnErr(ctx, expr); err != nil {
		return errors.Wrap(err, "unexpected error waiting for allocation")
	}
	if st.cooldown > 0 {
		if err := testing.Sleep(ctx, st.cooldown); err != nil {
			return errors.Wrap(err, "failed to sleep for cooldown")
		}
	}
	return nil
}

// Close closes the memory stress allocation tab.
func (st *MemoryStressUnit) Close(ctx context.Context, cr *chrome.Chrome) error {
	if st.conn == nil {
		return nil
	}
	st.conn.Close()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrapf(err, "failed to get TestAPIConn to close %q", st.url)
	}
	if err := tconn.Call(ctx, nil, `async (url) => {
		const query = tast.promisify(chrome.tabs.query);
		const remove = tast.promisify(chrome.tabs.remove);
		const tabs = await query({ url });
		// Works for any number of tabs, even if it will usually be 1.
		await Promise.all(tabs.map(t => remove(t.id)));
	}`, st.url); err != nil {
		return errors.Wrapf(err, "failed to close tab %q", st.url)
	}
	return nil
}

// StillAlive uses Chrome's debug tools to determine if a tab has been killed.
// It has not been killed if it is still a target for debugging.
func (st *MemoryStressUnit) StillAlive(ctx context.Context, cr *chrome.Chrome) bool {
	available, err := cr.IsTargetAvailable(ctx, chrome.MatchTargetURL(st.url))
	return err == nil && available
}

// FillChromeOSMemory launches memory stress tabs until one is killed, filling
// up memory in ChromeOS.
func FillChromeOSMemory(ctx context.Context, dataFileSystem http.FileSystem, cr *chrome.Chrome, unitMiB int, ratio float32) (func(context.Context) error, error) {
	server := NewMemoryStressServer(dataFileSystem)
	var units []*MemoryStressUnit
	cleanup := func(ctx context.Context) error {
		var res error
		for _, unit := range units {
			if err := unit.Close(ctx, cr); err != nil {
				testing.ContextLogf(ctx, "Failed to close MemoryStressUnit: %s", err)
				if res == nil {
					res = err
				}
			}
		}
		server.Close()
		return res
	}
	for i := 0; ; i++ {
		const tabOpenCooldown = 2 * time.Second
		unit := server.NewMemoryStressUnit(unitMiB, ratio, tabOpenCooldown, false)
		units = append(units, unit)
		if err := unit.Run(ctx, cr); err != nil {
			return cleanup, errors.Wrapf(err, "failed to run MemoryStressUnit %q", unit.url)
		}
		for _, unit := range units {
			if !unit.StillAlive(ctx, cr) {
				testing.ContextLogf(ctx, "FillChromeOSMemory started %d units of %d MiB before first kill", len(units), unitMiB)
				return cleanup, nil
			}
		}
	}
}

// MemoryStressTask wraps MemoryStressUnit to conform to the MemoryTask and
// KillableTask interfaces.
type MemoryStressTask struct{ MemoryStressUnit }

// MemoryStressTask is a MemoryTask.
var _ MemoryTask = (*MemoryStressTask)(nil)

// MemoryStressTask is a KillableTask.
var _ KillableTask = (*MemoryStressTask)(nil)

// String returns a friendly name for the task.
func (st *MemoryStressTask) String() string {
	return "Chrome Memory Stress Basic"
}

// NeedVM is false because we do not need Crostini.
func (st *MemoryStressTask) NeedVM() bool {
	return false
}

// Run creates a Chrome tab that allocates memory, then waits for the provided
// cooldown.
func (st *MemoryStressTask) Run(ctx context.Context, testEnv *TestEnv) error {
	return st.MemoryStressUnit.Run(ctx, testEnv.cr)
}

// Close closes the memory stress allocation tab.
func (st *MemoryStressTask) Close(ctx context.Context, testEnv *TestEnv) {
	st.MemoryStressUnit.Close(ctx, testEnv.cr)
}

// StillAlive returns false if the tab has been discarded, or was never opened.
func (st *MemoryStressTask) StillAlive(ctx context.Context, testEnv *TestEnv) bool {
	return st.MemoryStressUnit.StillAlive(ctx, testEnv.cr)
}

// MemoryStressServer is an http server that hosts the html and js needed to
// create MemoryStressTasks.
type MemoryStressServer struct {
	server *httptest.Server
	nextID int
}

// Resources needed by MemoryStressServer to create MemoryStressTasks.
const (
	AllocPageFilename  = "memory_stress.html"
	JavascriptFilename = "memory_stress.js"
)

// NewMemoryStressServer creates a server that can create MemoryStressTasks.
// Close() should be called after use.
func NewMemoryStressServer(dataFileSystem http.FileSystem) *MemoryStressServer {
	return &MemoryStressServer{
		server: httptest.NewServer(http.FileServer(dataFileSystem)),
	}
}

// NewMemoryStressUnit creates a new MemoryStressUnit.
// allocMiB - The amount of memory the tab will allocate.
// ratio    - How compressible the allocated memory will be.
// cooldown - How long to wait after allocating before returning.
// background - if true the tab is opened as a background tab.
func (s *MemoryStressServer) NewMemoryStressUnit(allocMiB int, ratio float32, cooldown time.Duration, background bool) *MemoryStressUnit {
	url := fmt.Sprintf("%s/%s?alloc=%d&ratio=%.3f&id=%d", s.server.URL, AllocPageFilename, allocMiB, ratio, s.nextID)
	s.nextID++
	return &MemoryStressUnit{
		url:        url,
		conn:       nil,
		cooldown:   cooldown,
		background: background,
	}
}

// NewMemoryStressTask creates a new MemoryStressTask.
// allocMiB - The amount of memory the tab will allocate.
// ratio    - How compressible the allocated memory will be.
// cooldown - How long to wait after allocating before returning.
func (s *MemoryStressServer) NewMemoryStressTask(allocMiB int, ratio float32, cooldown time.Duration) *MemoryStressTask {
	return &MemoryStressTask{*s.NewMemoryStressUnit(allocMiB, ratio, cooldown, false)}
}

// Close shuts down the http server.
func (s *MemoryStressServer) Close() {
	s.server.Close()
}
