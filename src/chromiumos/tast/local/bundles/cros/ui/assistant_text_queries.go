// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
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
		Func:         AssistantTextQueries,
		Desc:         "Tests Assistant's responses to text queries",
		Contacts:     []string{"meilinw@chromium.org", "xiaohuic@chromium.org"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome", "chrome_internal", "audio_play"},
	})
}

func AssistantTextQueries(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.ExtraArgs("--enable-features=ChromeOSAssistant"))
	if err != nil {
		s.Fatal("Failed to log in: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	// Starts Assistant service.
	if err := assistant.Enable(ctx, tconn); err != nil {
		s.Fatal("Failed to enable Assistant: ", err)
	}

	// Allows up to 20 seconds after the service bring-up for Libassistant to be fully started.
	// TODO(b/129896357): Replace the waiting logic once Libassistant has a reliable signal for
	// its readiness to watch for in the signed out mode.
	s.Log("Waiting for Assistant to be ready to answer queries")
	if err := waitForAssistant(ctx, tconn); err != nil {
		s.Error("Failed to wait for Libassistant to become ready: ", err)
	}

	testAssistantTimeQuery(ctx, tconn, s)
	testAssistantVolumeQueries(ctx, tconn, s)
}

// testAssistantTimeQuery verifies the correctness of the Assistant time response by parsing
// the fallback string to a time.Time object and comparing it with the current UTC time.
func testAssistantTimeQuery(ctx context.Context, tconn *chrome.Conn, s *testing.State) {
	s.Log("Sending time query to the Assistant")
	response, err := assistant.SendTextQuery(ctx, tconn, "what time is it in UTC?")
	if err != nil {
		s.Error("Failed to get Assistant time response: ", err)
		return
	}

	s.Log("Verifying the time query response")
	if len(response.Fallback) == 0 {
		s.Error("No htmlFallback field found in the response")
		return
	}

	now := time.Now().UTC()
	// Truncates the sec and nsec to be consistent with the format of assistantTime and
	// reduce the time error in interpretation.
	now = time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), 0, 0, now.Location())
	assistantTime, err := parseTimeNearNow(response.Fallback, now)
	if err != nil {
		s.Error("Failed to parse Assistant time response: ", err)
		return
	}

	// Two time results may not exactly match due to the response latency or clock drift, so we
	// instead check if |assistantTime| falls within an acceptable time range around |now|.
	tolerance := time.Minute
	earliest := now.Add(-tolerance)
	latest := now.Add(tolerance)
	if assistantTime.Before(earliest) || assistantTime.After(latest) {
		s.Errorf("Response time %v not within %v of current time %v",
			assistantTime, tolerance.Round(time.Second), now)
		return
	}
}

// parseTimeNearNow extracts the numeric components in the fallback string to construct a time
// object based on now time. For this particular query, a typical fallback text will have format:
// "It is (HH/H):MM AM/PM in the Coordinated Universal Time zone.".
func parseTimeNearNow(fallback string, now time.Time) (time.Time, error) {
	re := regexp.MustCompile(`(\d{1,2})(:\d\d)? ([AaPp][Mm])`)
	matches := re.FindStringSubmatch(fallback)
	if matches == nil {
		return time.Time{}, errors.New("fallback doesn't contain any well-formatted time results")
	}

	// Parses hours and minutes from the time string.
	hrs, err := strconv.Atoi(matches[1])
	if err != nil {
		return time.Time{}, err
	}
	var min int
	if len(matches[2]) != 0 {
		min, err = strconv.Atoi(matches[2][1:])
		if err != nil {
			return time.Time{}, err
		}
	}

	// Converts the time response to 24-hour clock to be consistent with the system time.
	if strings.EqualFold(matches[3], "AM") {
		if hrs == 12 {
			hrs -= 12
		}
	} else if strings.EqualFold(matches[3], "PM") {
		if hrs != 12 {
			hrs += 12
		}
	}

	// Because the fallback string only contains HH:MM, we choose the interpretation closest to
	// the now time.
	assistantTime := time.Date(now.Year(), now.Month(), now.Day(), hrs, min, 0, 0, now.Location())
	if diff := assistantTime.Sub(now); diff > 12*time.Hour {
		assistantTime.AddDate(0, 0, -1)
	} else if diff < -12*time.Hour {
		assistantTime.AddDate(0, 0, 1)
	}
	return assistantTime, nil
}

// testAssistantVolumeQueries tests setting and increasing volume actions via Assistant.
func testAssistantVolumeQueries(ctx context.Context, tconn *chrome.Conn, s *testing.State) {
	// Verifies the output stream nodes exist and are active before testing the volume queries.
	if err := audio.WaitForDevice(ctx, audio.OutputStream); err != nil {
		s.Error("No output stream nodes available: ", err)
		return
	}

	const testVolume = 25
	// Sends set volume query and verifies the result.
	s.Log("Sending set volume query to the Assistant")
	_, err := assistant.SendTextQuery(ctx, tconn, fmt.Sprintf("set volume to %d percent.", testVolume))
	if err != nil {
		s.Errorf("Failed to set volume to %d via Assistant: %v", testVolume, err)
		return
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
		s.Error("Timed out waiting for volume set: ", err)
		return
	}

	// Sends increase volume query and verifies the result.
	s.Log("Sending increase volume query to the Assistant")
	_, err = assistant.SendTextQuery(ctx, tconn, "turn up volume.")
	if err != nil {
		s.Error("Failed to increase volume via Assistant: ", err)
		return
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
		s.Error("Timed out waiting for volume increase: ", err)
		return
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

// waitForAssistant waits for the Assistant service to be fully ready by repeatedly sending
// the simple time query and checks for a nil error.
func waitForAssistant(ctx context.Context, tconn *chrome.Conn) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		_, err := assistant.SendTextQuery(ctx, tconn, "What's the time?")
		return err
	}, &testing.PollOptions{Timeout: 20 * time.Second})
}
