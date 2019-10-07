// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/assistant"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AssistantVolumeQueries,
		Desc:         "Tests setting and increasing volume actions via Assistant",
		Contacts:     []string{"meilinw@chromium.org", "xiaohuic@chromium.org"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome", "chrome_internal", "audio_play"},
		Pre:          chrome.LoggedIn(),
	})
}

func AssistantVolumeQueries(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	// Starts Assistant service.
	if err := assistant.Enable(ctx, tconn); err != nil {
		s.Fatal("Failed to enable Assistant: ", err)
	}

	// TODO(b/129896357): Replace the waiting logic once Libassistant has a reliable signal for
	// its readiness to watch for in the signed out mode.
	s.Log("Waiting for Assistant to be ready to answer queries")
	if err := assistant.WaitForServiceReady(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for Libassistant to become ready: ", err)
	}

	// Verifies the output stream nodes exist and are active before testing the volume queries.
	if err := audio.WaitForDevice(ctx, audio.OutputStream); err != nil {
		s.Fatal("No output stream nodes available: ", err)
	}

	const testVolume = 25
	s.Log("Sending set volume query to the Assistant")
	if _, err := assistant.SendTextQuery(ctx, tconn, fmt.Sprintf("set volume to %d percent.", testVolume)); err != nil {
		s.Fatalf("Failed to set volume to %d via Assistant: %v", testVolume, err)
	}

	s.Log("Verifying set volume query result")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		v, err := getActiveNodeVolume(ctx)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get system volume"))
		}
		if v != testVolume {
			return errors.Errorf("system volume %d doesn't match the requested volume %d", v, testVolume)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		s.Fatal("Timed out waiting for volume set: ", err)
	}

	s.Log("Sending increase volume query to the Assistant")
	if _, err := assistant.SendTextQuery(ctx, tconn, "turn up volume."); err != nil {
		s.Fatal("Failed to increase volume via Assistant: ", err)
	}

	s.Log("Verifying increase volume query result")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		v, err := getActiveNodeVolume(ctx)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get system volume"))
		}
		if v <= testVolume {
			return errors.Errorf("system volume doesn't increase: current - %d, base - %d", v, testVolume)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		s.Fatal("Timed out waiting for volume increase: ", err)
	}
}

// getActiveNodeVolume returns the current active node volume, ranging from 0 to 100.
func getActiveNodeVolume(ctx context.Context) (uint64, error) {
	// Turn on a display to re-enable an internal speaker on monroe.
	if err := power.TurnOnDisplay(ctx); err != nil {
		return 0, err
	}
	cras, err := audio.NewCras(ctx)
	if err != nil {
		return 0, err
	}
	nodes, err := cras.GetNodes(ctx)
	if err != nil {
		return 0, err
	}

	for _, n := range nodes {
		if n.Active && !n.IsInput {
			return n.NodeVolume, nil
		}
	}
	return 0, errors.New("cannot find active node volume from nodes")
}
