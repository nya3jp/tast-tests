// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package play provides common codes for video.Play* tests.
package play

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

type dataDir testing.State

func (d *dataDir) Open(name string) (http.File, error) {
	return os.Open((*testing.State)(d).DataPath(name))
}

// TestPlay checks that the video file named filename can be played back.
func TestPlay(s *testing.State, filename string) {
	ctx := s.Context()

	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	server := httptest.NewServer(http.FileServer((*dataDir)(s)))
	defer server.Close()

	conn, err := cr.NewConn(s.Context(), server.URL+"/video.html")
	if err != nil {
		s.Fatal("Creating renderer failed: ", err)
	}
	defer conn.Close()

	if err := conn.WaitForExpr(ctx, "script_ready"); err != nil {
		s.Fatal("Timed out waiting for player ready: ", err)
	}

	conn.Eval(ctx, fmt.Sprintf("loadVideoSource('%s')", filename), nil)

	if err := conn.WaitForExpr(ctx, "canplay()"); err != nil {
		s.Fatal("Timed out waiting for video load: ", err)
	}

	conn.Eval(ctx, "play()", nil)

	if err := conn.WaitForExpr(ctx, "currentTime() > 0.9"); err != nil {
		s.Fatal("Timed out waiting for playback: ", err)
	}

	conn.Eval(ctx, "pause()", nil)
}
