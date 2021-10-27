// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package browser

import (
	"context"

	"android.googlesource.com/platform/external/perfetto/protos/perfetto/trace"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/chrome/internal/driver"
	"chromiumos/tast/local/chrome/lacros/launcher"
)

type Type string

const (
	TypeAsh    Type = "ash"
	TypeLacros Type = "lacros"
)

type Browser struct {
	sess     *driver.Session
	closeFun func(ctx context.Context)
}

func fromAsh(c *chrome.Chrome) *Browser {
	return &Browser{c.Sess(), func(ctx context.Context) {}}
}
func fromLacros(l *launcher.LacrosChrome) *Browser {
	return &Browser{l.Sess(), func(ctx context.Context) { l.Close(ctx) }}
}

func Setup(ctx context.Context, f interface{}, typ Type) (*Browser, error) {
	switch typ {
	case TypeAsh:
		cr := f.(chrome.HasChrome).Chrome()
		return fromAsh(cr), nil
	case TypeLacros:
		f := f.(launcher.FixtValue)
		l, err := launcher.LaunchLacrosChrome(ctx, f)
		if err != nil {
			return nil, errors.Wrap(err, "failed to launch lacros-chrome")
		}
		return fromLacros(l), nil
	default:
		return nil, errors.Errorf("unrecognized Chrome type %s", string(typ))
	}
}

func (b *Browser) Close(ctx context.Context) {
	b.closeFun(ctx)
	b.closeFun = nil
	b.sess = nil
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
