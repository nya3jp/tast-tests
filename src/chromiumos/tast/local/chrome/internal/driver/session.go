// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package driver

import (
	"context"
	"fmt"

	"android.googlesource.com/platform/external/perfetto/protos/perfetto/trace"
	"github.com/mafredri/cdp/protocol/target"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/chrome/internal/browserwatcher"
	"chromiumos/tast/local/chrome/jslog"
	"chromiumos/tast/testing"
)

// Session allows interacting with a locally running Chrome.
//
// Session maintains a DevTools connection to Chrome. It also monitors a Chrome
// process with the browser watcher, as well as collecting JavaScript logs with
// jslog.
//
// Session is similar to chrome.Chrome, but it has following notable
// differences:
//
//  - Session interacts with a Chrome process already set up for debugging.
//    It is out of its scope to set up / start Chrome processes.
//  - A Session instance is tied to lifetime of a Chrome process. It maintains
//    states that would be cleared on restarting Chrome. A Session instance
//    cannot be reused for two different Chrome processes.
type Session struct {
	devsess *cdputil.Session
	watcher *browserwatcher.Watcher
	agg     *jslog.Aggregator // not owned

	tracingStarted bool
}

// NewSession connects to a local Chrome process and creates a new Session.
func NewSession(ctx context.Context, debuggingPortPath string, portWait cdputil.PortWaitOption, getBrowserPID func() (int, error), agg *jslog.Aggregator) (cr *Session, retErr error) {
	watcher := browserwatcher.NewWatcher(getBrowserPID)
	defer func() {
		if retErr != nil {
			watcher.Close()
		}
	}()

	devsess, err := cdputil.NewSession(ctx, debuggingPortPath, portWait)
	if err != nil {
		return nil, errors.Wrapf(watcher.ReplaceErr(err), "failed to establish connection to Chrome Debuggin Protocol with debugging port path=%q", cdputil.DebuggingPortPath)
	}
	defer func() {
		if retErr != nil {
			devsess.Close(ctx)
		}
	}()

	return &Session{
		devsess:        devsess,
		watcher:        watcher,
		agg:            agg,
		tracingStarted: false,
	}, nil
}

// Close releases resources associated to this object.
func (s *Session) Close(ctx context.Context) error {
	s.devsess.Close(ctx)
	return s.watcher.Close()
}

// DebugAddrPort returns the addr:port at which Chrome is listening for DevTools connections,
// e.g. "127.0.0.1:38725". This port should not be accessed from outside of this package,
// but it is exposed so that the port's owner can be easily identified.
func (s *Session) DebugAddrPort() string {
	return s.devsess.DebugAddrPort()
}

// Watcher returns the browser watcher associated with the session.
func (s *Session) Watcher() *browserwatcher.Watcher {
	return s.watcher
}

// JSLogAggregator returns the JavaScript log aggregated associated with the session.
func (s *Session) JSLogAggregator() *jslog.Aggregator {
	return s.agg
}

// NewConn creates a new Chrome renderer and returns a connection to it.
// If url is empty, an empty page (about:blank) is opened. Otherwise, the page
// from the specified URL is opened. You can assume that the page loading has
// been finished when this function returns.
func (s *Session) NewConn(ctx context.Context, url string, opts ...cdputil.CreateTargetOption) (*Conn, error) {
	if url == "" {
		testing.ContextLog(ctx, "Creating new blank page")
	} else {
		testing.ContextLog(ctx, "Creating new page with URL ", url)
	}
	targetID, err := s.devsess.CreateTarget(ctx, url, opts...)
	if err != nil {
		return nil, err
	}

	conn, err := s.newConnInternal(ctx, targetID, url)
	if err != nil {
		return nil, err
	}
	const blankURL = "about:blank"
	if url != "" && url != blankURL {
		if err := conn.WaitForExpr(ctx, fmt.Sprintf("location.href !== %q", blankURL)); err != nil {
			return nil, errors.Wrap(err, "failed to wait for navigation")
		}
	}
	if err := conn.WaitForExpr(ctx, "document.readyState === 'complete'"); err != nil {
		return nil, errors.Wrap(err, "failed to wait for loading")
	}
	return conn, nil
}

// newConnInternal is a convenience function that creates a new Conn connected to the specified target.
// url is only used for logging JavaScript console messages.
func (s *Session) newConnInternal(ctx context.Context, id TargetID, url string) (*Conn, error) {
	return NewConn(ctx, s.devsess, id, s.agg, url, s.watcher.ReplaceErr)
}

// Target describes a DevTools target.
type Target = target.Info

// TargetID is an ID assigned to a DevTools target.
type TargetID = target.ID

// TargetMatcher is a caller-provided function that matches targets with specific characteristics.
type TargetMatcher = cdputil.TargetMatcher

// MatchTargetID returns a TargetMatcher that matches targets with the supplied ID.
func MatchTargetID(id TargetID) TargetMatcher {
	return func(t *Target) bool { return t.TargetID == id }
}

// MatchTargetURL returns a TargetMatcher that matches targets with the supplied URL.
func MatchTargetURL(url string) TargetMatcher {
	return func(t *Target) bool { return t.URL == url }
}

// NewConnForTarget iterates through all available targets and returns a connection to the
// first one that is matched by tm. It polls until the target is found or ctx's deadline expires.
// An error is returned if no target is found, tm matches multiple targets, or the connection cannot
// be established.
//
//	f := func(t *Target) bool { return t.URL == "http://example.net/" }
//	conn, err := cr.NewConnForTarget(ctx, f)
func (s *Session) NewConnForTarget(ctx context.Context, tm TargetMatcher) (*Conn, error) {
	t, err := s.devsess.WaitForTarget(ctx, tm)
	if err != nil {
		return nil, s.watcher.ReplaceErr(err)
	}
	return s.newConnInternal(ctx, t.TargetID, t.URL)
}

// FindTargets returns the info about Targets, which satisfies the given cond condition.
func (s *Session) FindTargets(ctx context.Context, tm TargetMatcher) ([]*Target, error) {
	return s.devsess.FindTargets(ctx, tm)
}

// CloseTarget closes the target identified by the given id.
func (s *Session) CloseTarget(ctx context.Context, id TargetID) error {
	return s.devsess.CloseTarget(ctx, id)
}

// StartTracing starts trace events collection for the selected categories. Android
// categories must be prefixed with "disabled-by-default-android ", e.g. for the
// gfx category, use "disabled-by-default-android gfx", including the space.
// Note: StopTracing should be called even if StartTracing returns an error.
// Sometimes, the request to start tracing reaches the browser process, but there
// is a timeout while waiting for the reply.
func (s *Session) StartTracing(ctx context.Context, categories []string) error {
	// Note: even when StartTracing fails, it might be due to the case that the
	// StartTracing request is successfully sent to the browser and tracing
	// collection has started, but the context deadline is exceeded before Tast
	// receives the reply.  Therefore, tracingStarted flag is marked beforehand.
	s.tracingStarted = true
	return s.devsess.StartTracing(ctx, categories)
}

// StopTracing stops trace collection and returns the collected trace events.
func (s *Session) StopTracing(ctx context.Context) (*trace.Trace, error) {
	traces, err := s.devsess.StopTracing(ctx)
	if err != nil {
		return nil, err
	}
	s.tracingStarted = false
	return traces, nil
}

// TracingStarted returns whether tracing has started.
func (s *Session) TracingStarted() bool {
	return s.tracingStarted
}
