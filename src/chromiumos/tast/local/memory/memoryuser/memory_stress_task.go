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

// KillableTask allows querying whether a task has been killed or not.
type KillableTask interface {
	StillAlive(context.Context, *TestEnv) bool
}

// MemoryStressTask creates a Chrome tab that allocates memory like the
// platform.MemoryStressBasic test.
type MemoryStressTask struct {
	url   string
	conn  *chrome.Conn
	limit memory.Limit
}

// MemoryStressTask is a MemoryTask.
var _ MemoryTask = (*MemoryStressTask)(nil)

// Run creates a Chrome tab that allocates memory. If a memory.Limit has been
// provided, we wait until we are no longer limited.
func (st *MemoryStressTask) Run(ctx context.Context, testEnv *TestEnv) error {
	conn, err := testEnv.cr.NewConn(ctx, st.url)
	if err != nil {
		return errors.New("failed to open MemoryStressTask page")
	}
	st.conn = conn

	// Wait for allocation to complete.
	testing.ContextLogf(ctx, "Waiting for MemoryStressTask %q to allocate", st.url)
	const expr = "document.hasOwnProperty('out') == true"
	if err := conn.WaitForExprFailOnErr(ctx, expr); err != nil {
		return errors.Wrap(err, "unexpected error waiting for allocation")
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
func (st *MemoryStressTask) Close(ctx context.Context, testEnv *TestEnv) {
	if st.conn == nil {
		return
	}
	// Close the tab.
	st.conn.CloseTarget(ctx)
	st.conn.Close()
}

// StillAlive uses Chrome's debug tools to determine if a tab has been killed.
// It has not been killed if it is still a target for debugging.
func (st *MemoryStressTask) StillAlive(ctx context.Context, testEnv *TestEnv) bool {
	available, err := testEnv.cr.IsTargetAvailable(ctx, chrome.MatchTargetURL(st.url))
	return err == nil && available
}

// String returns a friendly name for the task.
func (st *MemoryStressTask) String() string {
	return "Chrome Memory Stress Basic"
}

// NeedVM is false because we do not need Crostini.
func (st *MemoryStressTask) NeedVM() bool {
	return false
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

// NewMemoryStressTask creates a new MemoryStressTask.
// allocMiB - The amount of memory the tab will allocate.
// ratio    - How compressible the allocated memory will be.
// limit    - (optional) wait until memory is not low after creating the tab.
func (s *MemoryStressServer) NewMemoryStressTask(allocMiB int, ratio float32, limit memory.Limit) *MemoryStressTask {
	url := fmt.Sprintf("%s/%s?alloc=%d&ratio=%.3f&id=%d", s.server.URL, AllocPageFilename, allocMiB, ratio, s.nextID)
	s.nextID++
	return &MemoryStressTask{
		url:   url,
		conn:  nil,
		limit: limit,
	}
}

// Close shuts down the http server.
func (s *MemoryStressServer) Close() {
	s.server.Close()
}
