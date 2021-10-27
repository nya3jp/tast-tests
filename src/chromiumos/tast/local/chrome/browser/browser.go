// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package browser

import (
	"context"

	"android.googlesource.com/platform/external/perfetto/protos/perfetto/trace"

	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/chrome/internal/driver"
)

type Type string

const (
	TypeAsh    Type = "ash"
	TypeLacros Type = "lacros"
)

type Browser struct {
	sess *driver.Session
}

func New(sess *driver.Session) *Browser {
	return &Browser{sess}
}

func (b *Browser) NewConn(ctx context.Context, url string, opts ...cdputil.CreateTargetOption) (*driver.Conn, error) {
	return b.sess.NewConn(ctx, url, opts...)
}

func (b *Browser) NewConnForTarget(ctx context.Context, tm driver.TargetMatcher) (*driver.Conn, error) {
	return b.sess.NewConnForTarget(ctx, tm)
}

func (b *Browser) TestAPIConn(ctx context.Context) (*driver.TestConn, error) {
	return b.sess.TestAPIConn(ctx)
}

func (b *Browser) StartTracing(ctx context.Context, categories []string, opts ...cdputil.TraceOption) error {
	return b.sess.StartTracing(ctx, categories, opts...)
}

func (b *Browser) StopTracing(ctx context.Context) (*trace.Trace, error) {
	return b.sess.StopTracing(ctx)
}
