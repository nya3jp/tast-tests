// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"
	"time"

	"android.googlesource.com/platform/external/perfetto/protos/perfetto/trace/github.com/google/perfetto/perfetto_proto"
	"github.com/mafredri/cdp/protocol/target"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/internal/cdputil"
	"chromiumos/tast/local/chrome/internal/driver"
	"chromiumos/tast/local/chrome/jslog"
	"chromiumos/tast/testing"
)

// UserDataDir is the directory that contains the user data of lacros.
const UserDataDir = "/home/chronos/user/lacros/"

// Lacros contains all state associated with a lacros-chrome instance
// that has been launched. Must call Close() to release resources.
type Lacros struct {
	lacrosPath  string // Root directory for lacros-chrome.
	userDataDir string // User data directory

	cmd  *testexec.Cmd // The command context used to start lacros-chrome.
	agg  *jslog.Aggregator
	sess *driver.Session // Debug session connected lacros-chrome.
}

// Browser returns a Browser instance.
func (l *Lacros) Browser() *browser.Browser {
	return browser.New(l.sess, browser.Closer(l.Close))
}

// StartTracing starts trace events collection for the selected categories. Android
// categories must be prefixed with "disabled-by-default-android ", e.g. for the
// gfx category, use "disabled-by-default-android gfx", including the space.
// This must not be called after Close().
func (l *Lacros) StartTracing(ctx context.Context, categories []string, opts ...cdputil.TraceOption) error {
	return l.sess.StartTracing(ctx, categories, opts...)
}

// StartSystemTracing starts trace events collection from the system tracing
// service using the marshaled binary protobuf trace config.
// Note: StopTracing should be called even if StartTracing returns an error.
// Sometimes, the request to start tracing reaches the browser process, but there
// is a timeout while waiting for the reply.
func (l *Lacros) StartSystemTracing(ctx context.Context, perfettoConfig []byte) error {
	return l.sess.StartSystemTracing(ctx, perfettoConfig)
}

// StopTracing stops trace collection and returns the collected trace events.
// This must not be called after Close().
func (l *Lacros) StopTracing(ctx context.Context) (*perfetto_proto.Trace, error) {
	return l.sess.StopTracing(ctx)
}

// Close kills a launched instance of lacros-chrome.
func (l *Lacros) Close(ctx context.Context) error {
	if l.sess == nil {
		return errors.New("close should not be called on already closed session")
	}
	if err := l.sess.Close(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to close connection to lacros-chrome: ", err)
	}
	l.sess = nil
	l.agg.Close()
	l.agg = nil

	if l.cmd != nil {
		if err := l.cmd.Kill(); err != nil {
			testing.ContextLog(ctx, "Failed to kill lacros-chrome: ", err)
		}
		l.cmd.Wait()
		l.cmd = nil
	}

	if err := killLacros(ctx, l.lacrosPath); err != nil {
		return errors.Wrap(err, "failed to kill lacros-chrome")
	}
	return nil
}

// NewConnForTarget iterates through all available targets and returns a connection to the
// first one that is matched by tm.
// This must not be called after Close().
func (l *Lacros) NewConnForTarget(ctx context.Context, tm chrome.TargetMatcher) (*chrome.Conn, error) {
	return l.sess.NewConnForTarget(ctx, tm)
}

// FindTargets returns the info about Targets, which satisfies the given cond condition.
// This must not be called after Close().
func (l *Lacros) FindTargets(ctx context.Context, tm chrome.TargetMatcher) ([]*chrome.Target, error) {
	return l.sess.FindTargets(ctx, tm)
}

// NewConn creates a new Chrome renderer and returns a connection to it.
// If url is empty, an empty page (about:blank) is opened. Otherwise, the page
// from the specified URL is opened. You can assume that the page loading has
// been finished when this function returns.
// This must not be called after Close().
func (l *Lacros) NewConn(ctx context.Context, url string, opts ...cdputil.CreateTargetOption) (*chrome.Conn, error) {
	return l.sess.NewConn(ctx, url, opts...)
}

// TestAPIConn returns a new chrome.TestConn instance for the lacros browser.
// This must not be called after Close().
func (l *Lacros) TestAPIConn(ctx context.Context) (*chrome.TestConn, error) {
	return l.sess.TestAPIConn(ctx)
}

// CloseAboutBlank finds all targets that are about:blank, closes them, then waits until they are gone.
// windowsExpectedClosed indicates how many windows that we expect to be closed from doing this operation.
// This takes *ash-chrome*'s TestConn as tconn, not the one provided by Lacros.TestAPIConn.
func (l *Lacros) CloseAboutBlank(ctx context.Context, tconn *chrome.TestConn, windowsExpectedClosed int) error {
	prevWindows, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		return err
	}

	targets, err := l.sess.FindTargets(ctx, driver.MatchTargetURL(chrome.BlankURL))
	if err != nil {
		return errors.Wrap(err, "failed to query for about:blank pages")
	}
	allPages, err := l.sess.FindTargets(ctx, func(t *target.Info) bool { return t.Type == "page" })
	if err != nil {
		return errors.Wrap(err, "failed to query for all pages")
	}

	for _, info := range targets {
		if err := l.sess.CloseTarget(ctx, info.TargetID); err != nil {
			return err
		}
	}
	return testing.Poll(ctx, func(ctx context.Context) error {
		// If we are closing all lacros targets, then lacros Chrome will exit. In that case, we won't be able to
		// communicate with it, so skip checking the targets. Since closing all lacros targets will close all
		// lacros windows, the window check below is necessary and sufficient.
		if len(targets) != len(allPages) {
			targets, err := l.sess.FindTargets(ctx, driver.MatchTargetURL(chrome.BlankURL))
			if err != nil {
				return testing.PollBreak(err)
			}
			if len(targets) != 0 {
				return errors.New("not all about:blank targets were closed")
			}
		}

		windows, err := ash.GetAllWindows(ctx, tconn)
		if err != nil {
			return testing.PollBreak(err)
		}
		if len(prevWindows)-len(windows) != windowsExpectedClosed {
			return errors.Errorf("expected %d windows to be closed, got %d closed",
				windowsExpectedClosed, len(prevWindows)-len(windows))
		}

		return nil
	}, &testing.PollOptions{Timeout: time.Minute})
}
