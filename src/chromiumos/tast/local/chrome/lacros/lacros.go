// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"

	"android.googlesource.com/platform/external/perfetto/protos/perfetto/trace/github.com/google/perfetto/perfetto_proto"
	"github.com/mafredri/cdp/protocol/target"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/internal/cdputil"
	"chromiumos/tast/local/chrome/internal/driver"
	"chromiumos/tast/local/chrome/jslog"
)

// UserDataDir is the directory that contains the user data of lacros.
const UserDataDir = "/home/chronos/user/lacros/"

// Lacros contains all state associated with a lacros-chrome instance
// that has been launched. Must call Close() to release resources.
type Lacros struct {
	agg  *jslog.Aggregator
	sess *driver.Session // Debug session connected lacros-chrome.
}

// Browser returns a Browser instance.
func (l *Lacros) Browser() *browser.Browser {
	return browser.New(l.sess)
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

// Close closes all lacros chrome targets and the dev session.
func (l *Lacros) Close(ctx context.Context) error {
	// Get all pages. Note that we can't get all targets, because one of them
	// will be the test extension or devtools and we don't want to kill that.
	// Further note that this will mean pages are not restored, compared to killing
	// lacros directly.
	// TODO(crbug.com/1311504): There is similar functionality in chrome.ResetState. Integrate these?
	// TODO(crbug.com/1312306): For some reason, including t.Type == "other" breaks this.
	ts, err := l.sess.FindTargets(ctx, func(t *target.Info) bool {
		return t.Type == "page" || t.Type == "app"
	})
	if err != nil {
		return errors.Wrap(err, "failed to query for all targets")
	}

	for _, info := range ts {
		if err := l.sess.CloseTarget(ctx, info.TargetID); err != nil {
			return err
		}
	}

	// The browser may already be terminated by the time we try to close the
	// dev session, so ignore any error.
	l.sess.Close(ctx)
	l.sess = nil
	l.agg.Close()
	l.agg = nil

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
