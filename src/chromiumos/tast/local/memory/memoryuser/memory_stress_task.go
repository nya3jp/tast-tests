// Copyright 2020 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/local/memory"
	"chromiumos/tast/testing"
)

// MemoryStressUnit creates a Chrome tab that allocates memory like the
// platform.MemoryStressBasic test.
type MemoryStressUnit struct {
	url      string
	conn     *chrome.Conn
	limit    memory.Limit
	cooldown time.Duration
}

// Run creates a Chrome tab that allocates memory. If a memory.Limit has been
// provided, we wait until we are no longer limited.
func (st *MemoryStressUnit) Run(ctx context.Context, cr *chrome.Chrome) error {
	conn, err := cr.NewConn(ctx, st.url)
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
	if st.limit == nil {
		return nil
	}
	// Limit has been provided, wait until we are not limited.
	if err := testing.Poll(ctx, st.limit.AssertNotReached, &testing.PollOptions{Interval: 500 * time.Millisecond, Timeout: 10 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to wait for ChromeOS memory to not be above limit")
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
		unit := server.NewMemoryStressUnit(unitMiB, ratio, nil, 2*time.Second)
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

// Run creates a Chrome tab that allocates memory. If a memory.Limit has been
// provided, we wait until we are no longer limited.
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
// limit    - (optional) wait until memory is not low after creating the tab.
func (s *MemoryStressServer) NewMemoryStressUnit(allocMiB int, ratio float32, limit memory.Limit, cooldown time.Duration) *MemoryStressUnit {
	url := fmt.Sprintf("%s/%s?alloc=%d&ratio=%.3f&id=%d", s.server.URL, AllocPageFilename, allocMiB, ratio, s.nextID)
	s.nextID++
	return &MemoryStressUnit{
		url:      url,
		conn:     nil,
		limit:    limit,
		cooldown: cooldown,
	}
}

// NewMemoryStressTask creates a new MemoryStressTask.
// allocMiB - The amount of memory the tab will allocate.
// ratio    - How compressible the allocated memory will be.
// limit    - (optional) wait until memory is not low after creating the tab.
// cooldown - How long to wait after allocating before returning.
func (s *MemoryStressServer) NewMemoryStressTask(allocMiB int, ratio float32, limit memory.Limit, cooldown time.Duration) *MemoryStressTask {
	return &MemoryStressTask{*s.NewMemoryStressUnit(allocMiB, ratio, limit, cooldown)}
}

// Close shuts down the http server.
func (s *MemoryStressServer) Close() {
	s.server.Close()
}
