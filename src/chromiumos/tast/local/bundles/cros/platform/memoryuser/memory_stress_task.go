// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package memoryuser

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// TODO: copied from arc/memory/chromeos.go, put in some sommon lib

// readFirstUint reads the first unsigned integer from a file.
func readFirstUint(f string) (uint, error) {
	// Files will always just be a single line, so it's OK to read everything.
	data, err := ioutil.ReadFile(f)
	if err != nil {
		return 0, err
	}
	firstString := strings.Split(strings.TrimSpace(string(data)), " ")[0]
	firstUint, err := strconv.ParseUint(firstString, 10, 64)
	if err != nil {
		return 0, errors.Wrapf(err, "unable to convert %q to integer", data)
	}
	return uint(firstUint), nil
}

// ChromeOSAvailable reads available memory from sysfs.
// Returns available memory in MB.
func ChromeOSAvailable() (uint, error) {
	const availableMemorySysFile = "/sys/kernel/mm/chromeos-low_mem/available"
	return readFirstUint(availableMemorySysFile)
}

// ChromeOSCriticalMargin reads the critical margin from sysfs.
// Returns margin in MB.
func ChromeOSCriticalMargin() (uint, error) {
	const marginMemorySysFile = "/sys/kernel/mm/chromeos-low_mem/margin"
	return readFirstUint(marginMemorySysFile)
}

// KillableTask allows querying whether a task has been killed or not.
type KillableTask interface {
	StillAlive(context.Context, *TestEnv) bool
}

// MemoryStressTask creates a Chrome tab that allocates memory like the
// platform.MemoryStressBasic test.
type MemoryStressTask struct {
	url  string
	conn *chrome.Conn
}

var errStillCritical = errors.New("memory still critical")

func memoryNotCritical(_ctx context.Context) error {
	margin, err := ChromeOSCriticalMargin()
	if err != nil {
		return err
	}
	available, err := ChromeOSAvailable()
	if err != nil {
		return err
	}
	if available < margin {
		return errStillCritical
	}
	return nil
}

// Run creates a Chrome tab that allocates memory, and waits until ChromeOS's
// memory pressure is below critical.
func (st *MemoryStressTask) Run(ctx context.Context, s *testing.State, testEnv *TestEnv) error {
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

	// Wait until memory is not critical.
	if err := testing.Poll(ctx, memoryNotCritical, &testing.PollOptions{Interval: 500 * time.Millisecond, Timeout: 10 * time.Second}); err != nil {
		if errors.Is(err, errStillCritical) {
			testing.ContextLogf(ctx, "Memory still critical after memory stress task %q", st.url)
			return nil
		}
		return errors.Wrap(err, "failed to wait for ChromeOS memory to not be critical")
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
func NewMemoryStressServer(dataFileSystem http.FileSystem) *MemoryStressServer {
	return &MemoryStressServer{
		server: httptest.NewServer(http.FileServer(dataFileSystem)),
	}
}

// NewMemoryStressTask creates a new MemoryStressTask.
func (s *MemoryStressServer) NewMemoryStressTask(allocMb int, ratio float32) *MemoryStressTask {
	url := fmt.Sprintf("%s/%s?alloc=%d&ratio=%.3f&id=%d", s.server.URL, AllocPageFilename, allocMb, ratio, s.nextID)
	s.nextID++
	return &MemoryStressTask{
		url:  url,
		conn: nil,
	}
}

// Close shuts down the http server.
func (s *MemoryStressServer) Close() {
	s.server.Close()
}
