// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package play provides common codes for video.Play* tests.
package play

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/faillog"
	"chromiumos/tast/testing"
)

// dataDir implements http.FileSystem and is passed to http.FileServer to serve
// test data.
type DataDir testing.State

func (d *DataDir) Open(name string) (http.File, error) {
	// DataPath doesn't want a leading slash, so strip it off if present.
	if filepath.IsAbs(name) {
		var err error
		if name, err = filepath.Rel("/", name); err != nil {
			return nil, err
		}
	}
	// Chrome requests favicons. We don't register a favicon file, so avoid asking
	// DataPath for the path to it.
	if name == "favicon.ico" {
		return nil, errors.New("not found")
	}
	return os.Open((*testing.State)(d).DataPath(name))
}

// TestPlay checks that the video file named filename can be played back.
func TestPlay(s *testing.State, filename string) {
	defer faillog.SaveIfError(s)

	ctx := s.Context()

	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	server := httptest.NewServer(http.FileServer((*DataDir)(s)))
	defer server.Close()

	conn, err := cr.NewConn(s.Context(), server.URL+"/video.html")
	if err != nil {
		s.Fatal("Creating renderer failed: ", err)
	}
	defer conn.Close()

	if err := conn.WaitForExpr(ctx, "script_ready"); err != nil {
		s.Fatal("Timed out waiting for player ready: ", err)
	}

	if err := conn.Exec(ctx, fmt.Sprintf("loadVideoSource(%q)", filename)); err != nil {
		s.Fatal("Failed loadVideoSource: ", err)
	}

	if err := conn.WaitForExpr(ctx, "canplay()"); err != nil {
		s.Fatal("Timed out waiting for video load: ", err)
	}

	if err := conn.Exec(ctx, "play()"); err != nil {
		s.Fatal("Failed play: ", err)
	}

	if err := conn.WaitForExpr(ctx, "currentTime() > 0.9"); err != nil {
		s.Fatal("Timed out waiting for playback: ", err)
	}

	if err := conn.Exec(ctx, "pause()"); err != nil {
		s.Fatal("Failed pause: ", err)
	}
}
