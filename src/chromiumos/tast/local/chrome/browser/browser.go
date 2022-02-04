// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package browser implements a layer of abstraction over Ash and Lacros Chrome
// instances.
package browser

import (
	"context"

	"android.googlesource.com/platform/external/perfetto/protos/perfetto/trace/github.com/google/perfetto/perfetto_proto"

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

// Closer holds a weak reference to a function needed to close an underlying browser either ash-chrome or lacros-chrome.
// (Browser).Close will call the closer function.
type Closer func(ctx context.Context) error

// Browser consists of a Chrome session and a closer to be called in (Browser).Close.
type Browser struct {
	sess   *driver.Session
	closer Closer
}

// New creates a new Browser instance from an existing Chrome session.
func New(sess *driver.Session, closer Closer) *Browser {
	return &Browser{sess, closer}
}

// CreateTargetOption is cpdutil.CreateTargetOption.
type CreateTargetOption = cdputil.CreateTargetOption

// WithNewWindow behaves like cpdutil.WithNewWindow.
// TODO(neis): The other one, WithBackground, is unused. Can we get rid of the whole thing?
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

// TargetMatcher is chrome.TargetMatcher.
type TargetMatcher = driver.TargetMatcher

// NewConnForTarget iterates through all available targets and returns a connection to the
// first one that is matched by tm.
func (b *Browser) NewConnForTarget(ctx context.Context, tm TargetMatcher) (*Conn, error) {
	return b.sess.NewConnForTarget(ctx, tm)
}

// Close calls a closer function registered from a caller of New which is either lacros-chrome or ash-chrome.
func (b *Browser) Close(ctx context.Context) error {
	if b.closer != nil {
		// TODO: Remove log.
		testing.ContextLogf(ctx, "DEBUG: Closing browser. closer: %#v", b.closer)
		return b.closer(ctx)
	}
	return nil // No closer found skipping Close
}

// TestConn is chrome.TestConn.
type TestConn = driver.TestConn

// TestAPIConn returns a new TestConn instance.
func (b *Browser) TestAPIConn(ctx context.Context) (*TestConn, error) {
	return b.sess.TestAPIConn(ctx)
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
