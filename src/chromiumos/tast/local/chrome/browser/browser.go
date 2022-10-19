// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package browser implements a layer of abstraction over Ash and Lacros Chrome
// instances.
package browser

import (
	"context"
	"time"

	"android.googlesource.com/platform/external/perfetto/protos/perfetto/trace/github.com/google/perfetto/perfetto_proto"
	"github.com/mafredri/cdp/protocol/target"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/internal/cdputil"
	"chromiumos/tast/local/chrome/internal/driver"
	"chromiumos/tast/testing"
)

// Type indicates the type of Chrome browser to be used.
type Type string

const (
	// TypeAsh refers to Ash Chrome (the system browser).
	TypeAsh Type = "ash"
	// TypeLacros refers to Lacros Chrome (the user browser).
	TypeLacros Type = "lacros"
)

// Browser consists primarily of a Chrome session.
type Browser struct {
	sess                     *driver.Session
	autotestPrivateSupported bool
}

// New creates a new Browser instance from an existing Chrome session.
func New(sess *driver.Session, autotestPrivateSupported bool) *Browser {
	return &Browser{sess, autotestPrivateSupported}
}

// CreateTargetOption is cpdutil.CreateTargetOption.
type CreateTargetOption = cdputil.CreateTargetOption

// WithNewWindow behaves like cpdutil.WithNewWindow.
func WithNewWindow() CreateTargetOption {
	return cdputil.WithNewWindow()
}

// Conn is chrome.Conn
type Conn = driver.Conn

// NewConn creates a new Chrome renderer and returns a connection to it.
// If url is empty, an empty page (about:blank) is opened. Otherwise, the page
// from the specified URL is opened. You can assume that the page loading has
// been finished when this function returns.
func (b *Browser) NewConn(ctx context.Context, url string, opts ...CreateTargetOption) (*Conn, error) {
	return b.sess.NewConn(ctx, url, opts...)
}

// Target is chrome.Target.
type Target = driver.Target

// TargetID is chrome.TargetID.
type TargetID = driver.TargetID

// TargetMatcher is chrome.TargetMatcher.
type TargetMatcher = driver.TargetMatcher

// NewConnForTarget iterates through all available targets and returns a connection to the
// first one that is matched by tm.
func (b *Browser) NewConnForTarget(ctx context.Context, tm TargetMatcher) (*Conn, error) {
	return b.sess.NewConnForTarget(ctx, tm)
}

// FindTargets returns the info about Targets, which satisfies the given cond condition.
// This must not be called after Close().
func (b *Browser) FindTargets(ctx context.Context, tm TargetMatcher) ([]*Target, error) {
	return b.sess.FindTargets(ctx, tm)
}

// CloseTarget closes the target identified by the given id.
func (b *Browser) CloseTarget(ctx context.Context, id TargetID) error {
	return b.sess.CloseTarget(ctx, id)
}

// IsTargetAvailable checks if there is any matched target.
func (b *Browser) IsTargetAvailable(ctx context.Context, tm TargetMatcher) (bool, error) {
	targets, err := b.FindTargets(ctx, tm)
	if err != nil {
		return false, errors.Wrap(err, "failed to get targets")
	}
	return len(targets) != 0, nil
}

// TestConn is chrome.TestConn.
type TestConn = driver.TestConn

// TestAPIConn returns a new TestConn instance.
func (b *Browser) TestAPIConn(ctx context.Context) (*TestConn, error) {
	return b.sess.TestAPIConn(ctx, b.autotestPrivateSupported)
}

// TraceOption is cpdutil.TraceOption.
type TraceOption = cdputil.TraceOption

// DisableSystrace behaves like cpdutil.DisableSystrace.
func DisableSystrace() TraceOption {
	return cdputil.DisableSystrace()
}

// StartTracing starts trace events collection for the selected categories. Android
// categories must be prefixed with "disabled-by-default-android ", e.g. for the
// gfx category, use "disabled-by-default-android gfx", including the space.
func (b *Browser) StartTracing(ctx context.Context, categories []string, opts ...TraceOption) error {
	return b.sess.StartTracing(ctx, categories, opts...)
}

// StopTracing stops trace collection and returns the collected trace events.
func (b *Browser) StopTracing(ctx context.Context) (*perfetto_proto.Trace, error) {
	return b.sess.StopTracing(ctx)
}

// ReloadActiveTab reloads the active tab.
func (b *Browser) ReloadActiveTab(ctx context.Context) error {
	tconn, err := b.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create Test API connection")
	}
	if err := tconn.Eval(ctx, "chrome.tabs.reload()", nil); err != nil {
		return errors.Wrap(err, "failed to reload tab")
	}
	if err := tconn.WaitForExpr(ctx, "document.readyState === 'complete'"); err != nil {
		return errors.Wrap(err, "failed to wait for the ready state")
	}
	return nil
}

// CloseWithURL finds all targets with the given url, closes them, and waits
// until they are closed. Note that if this closes all lacros pages, lacros will
// exit, and we won't be able to verify closing was done successfully.
// If this turns out to cause flakes, we can additionally poll to see if
// the lacros process still exists, and if it does then poll each target
// to see if it closed.
func (b *Browser) CloseWithURL(ctx context.Context, url string) error {
	targets, err := b.sess.FindTargets(ctx, driver.MatchTargetURL(url))
	if err != nil {
		return errors.Wrap(err, "failed to query for about:blank pages")
	}

	allPages, err := b.sess.FindTargets(ctx, func(t *target.Info) bool { return t.Type == "page" })
	if err != nil {
		return errors.Wrap(err, "failed to query for all pages")
	}

	for _, info := range targets {
		if err := b.sess.CloseTarget(ctx, info.TargetID); err != nil {
			return err
		}
	}

	if len(targets) != len(allPages) {
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			targets, err := b.sess.FindTargets(ctx, driver.MatchTargetURL(url))
			if err != nil {
				return testing.PollBreak(err)
			}
			if len(targets) != 0 {
				return errors.New("not all about:blank targets were closed")
			}

			return nil
		}, &testing.PollOptions{Timeout: time.Minute}); err != nil {
			return err
		}
	}

	return nil

}

// CurrentTabs returns the tabs of the current window.
func (b *Browser) CurrentTabs(ctx context.Context) ([]Tab, error) {
	tconn, err := b.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create Test API connection")
	}
	return CurrentTabs(ctx, tconn)
}
