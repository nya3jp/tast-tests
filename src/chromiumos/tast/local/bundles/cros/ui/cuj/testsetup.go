// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cuj

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/ui/cujrecorder"
	"chromiumos/tast/testing"
)

type TastTest struct {
	Ctx             context.Context
	CloseCtx        context.Context
	Cr              *chrome.Chrome
	Cs              ash.ConnSource
	Lacros          *lacros.Lacros
	Tconn           *chrome.TestConn
	Btconn          *chrome.TestConn
	Recorder        *cujrecorder.Recorder
	KeyboardWriter  *input.KeyboardEventWriter
	TrackpadWriter  *input.TrackpadEventWriter
	TouchWriter     *input.TouchEventWriter
	DisplayInfo     *display.Info
	deferFns        []func()
	deferFnsWithCtx []func(context.Context) error
}

func Setup(ctx context.Context, s *testing.State, browserType *browser.Type) *TastTest {
	var deferFns []func()
	var deferFnsWithCtx []func(context.Context) error

	// Reserve ten seconds for cleanup.
	closeCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	deferFns = append(deferFns, cancel)

	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to the Ash TestAPIConn: ", err)
	}

	if browserType == nil {
		return &TastTest{
			Ctx:             ctx,
			CloseCtx:        closeCtx,
			Cr:              cr,
			Tconn:           tconn,
			deferFns:        deferFns,
			deferFnsWithCtx: deferFnsWithCtx,
		}
	}

	var cs ash.ConnSource
	var bTconn *chrome.TestConn
	var l *lacros.Lacros
	switch *browserType {
	case browser.TypeLacros:
		// Launch lacros.
		var err error
		l, err = lacros.Launch(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to launch Lacros: ", err)
		}
		deferFnsWithCtx = append(deferFnsWithCtx, l.Close)
		cs = l

		if bTconn, err = l.TestAPIConn(ctx); err != nil {
			s.Fatal("Failed to connect to the Lacros TestAPIConn: ", err)
		}
	case browser.TypeAsh:
		cs = cr
		bTconn = tconn
	}

	return &TastTest{
		Ctx:             ctx,
		CloseCtx:        closeCtx,
		Cr:              cr,
		Cs:              cs,
		Lacros:          l,
		Tconn:           tconn,
		Btconn:          bTconn,
		deferFns:        deferFns,
		deferFnsWithCtx: deferFnsWithCtx,
	}
}

func (t *TastTest) SetupRecorder(s *testing.State) *cujrecorder.Recorder {
	if t.Cr == nil {
		TestNotInitialized(s, "cr")
	}
	recorder, err := cujrecorder.NewRecorder(t.Ctx, t.Cr, nil, cujrecorder.RecorderOptions{})
	if err != nil {
		s.Fatal("Failed to create a new CUJ recorder: ", err)
	}
	t.Recorder = recorder
	t.deferFns = append(t.deferFns, func() {
		if err := recorder.Close(t.CloseCtx); err != nil {
			s.Error("Failed to stop recorder: ", err)
		}
	})
	return recorder
}

func (t *TastTest) SetupKeyboardEventWriter(s *testing.State) *input.KeyboardEventWriter {
	kw, err := input.Keyboard(t.Ctx)
	if err != nil {
		s.Fatal("Failed to create a keyboard: ", err)
	}
	t.KeyboardWriter = kw
	t.deferFns = append(t.deferFns, func() { kw.Close() })
	return kw
}

func (t *TastTest) SetupTrackpadEventWriter(s *testing.State) *input.TrackpadEventWriter {
	tpw, err := input.Trackpad(t.Ctx)
	if err != nil {
		s.Fatal("Failed to create a trackpad device: ", err)
	}
	t.TrackpadWriter = tpw
	t.deferFns = append(t.deferFns, func() { tpw.Close() })
	return tpw
}

func (t *TastTest) SetupMultiTouchWriter(s *testing.State, numTouches int) *input.TouchEventWriter {
	if t.TrackpadWriter == nil {
		t.SetupTrackpadEventWriter(s)
	}

	tw, err := t.TrackpadWriter.NewMultiTouchWriter(2)
	if err != nil {
		s.Fatal("Failed to create a multi touch writer: ", err)
	}
	t.TouchWriter = tw
	t.deferFns = append(t.deferFns, tw.Close)
	return tw
}

func (t *TastTest) SetupDisplayInfo(s *testing.State) *display.Info {
	if t.Tconn == nil {
		TestNotInitialized(s, "ctx, tconn")
	}
	info, err := display.GetPrimaryInfo(t.Ctx, t.Tconn)
	if err != nil {
		s.Fatal("Failed to get the primary display info: ", err)
	}
	t.DisplayInfo = info
	return info
}

func SetupIf(s *testing.State, setupFn func(s *testing.State), condition bool) {
	if condition {
		setupFn(s)
	}
}

func (t *TastTest) Cleanup() {
	for _, fn := range t.deferFns {
		fn()
	}
	for _, fn := range t.deferFnsWithCtx {
		fn(t.CloseCtx)
	}
}

func TestNotInitialized(s *testing.State, uninitializedVars string) {
	s.Fatalf("Failed to Setup recorder because %s was not initialized first", uninitializedVars)
}
