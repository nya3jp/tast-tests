// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"
	"path/filepath"
	"time"

	"android.googlesource.com/platform/external/perfetto/protos/perfetto/trace/github.com/google/perfetto/perfetto_proto"
	"github.com/mafredri/cdp/protocol/target"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/internal/cdputil"
	"chromiumos/tast/local/chrome/internal/driver"
	"chromiumos/tast/local/chrome/jslog"
	"chromiumos/tast/local/chrome/lacros/lacrosinfo"
	"chromiumos/tast/local/logsaver"
	"chromiumos/tast/testing"
)

// Lacros contains all state associated with a lacros-chrome instance
// that has been launched. Must call Close() to release resources.
type Lacros struct {
	agg    *jslog.Aggregator
	sess   *driver.Session  // Debug session connected lacros-chrome.
	ctconn *chrome.TestConn // Ash TestConn.

	logFilename string
	logMarker   *logsaver.Marker
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

// CloseResources closes lacros resources without closing targets.
// TODO(crbug.com/1318180): Instead we may want to change Lacros to use ResetState and Close fn
// like Chrome, or provide these functions on the Browser().
func (l *Lacros) CloseResources(ctx context.Context) {
	l.sess.Close(ctx)
	l.sess = nil
	l.agg.Close()
	l.agg = nil
}

// Close closes all lacros chrome targets and the dev session.
func (l *Lacros) Close(ctx context.Context) error {
	// Save the entire lacros log to outDir. 
	if outDir, ok := testing.ContextOutDir(ctx); ok {
		if err := l.logMarker.Save(filepath.Join(outDir, "lacros.log")); err != nil {
			testing.ContextLog(ctx, "Failed to save the entire lacros log: ", err)
		}
	} else {
		testing.ContextLog(ctx, "No output directory exists, not saving log file")
	}
	testing.ContextLog(ctx, "Lacros log file saved")

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

	info, err := lacrosinfo.Snapshot(ctx, l.ctconn)
	if err != nil {
		return errors.Wrap(err, "failed to get lacros info")
	}

	var sessErr error
	for _, t := range ts {
		if err := l.sess.CloseTarget(ctx, t.TargetID); err != nil {
			// CloseTarget should not error if keep-alive is on, since the browser
			// won't die.
			if info.KeepAlive {
				return errors.Wrap(err, "failed to close target")
			}
			// Otherwise, ignore the error here, as closing the target may close
			// lacros and cause an error.
			if sessErr != nil {
				sessErr = err
			}
		}
	}

	// If keep-alive is on, then if there was an error closing targets we would
	// have returned it. If keep-alive is off, then we will expect lacros to be
	// stopped soon if the error was due to CloseTarget trying to run on a
	// closed browser. So, poll to make sure lacros actually closes.
	if !info.KeepAlive {
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			info, err := lacrosinfo.Snapshot(ctx, l.ctconn)
			if err != nil {
				return testing.PollBreak(errors.Wrap(err, "failed to get lacros info"))
			}
			if info.State != lacrosinfo.LacrosStateStopped {
				return errors.Wrap(err, "lacros not yet stopped")
			}
			return nil
		}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
			return errors.Wrap(sessErr, "lacros unexpectedly not yet stopped")
		}
	}

	// The browser may already be terminated by the time we try to close the
	// dev session, so ignore any error.
	l.CloseResources(ctx)

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
