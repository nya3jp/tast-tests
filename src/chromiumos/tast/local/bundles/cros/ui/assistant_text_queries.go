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
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AssistantTextQueries,
		Desc:         "Tests Assistant's responses to text queries",
		Contacts:     []string{"meilinw@chromium.org", "xiaohuic@chromium.org"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome_login", "chrome_internal"},
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

	// Tests time query and verifies the result.
	testAssistantTimeQuery(ctx, tconn, s)

	// Tests volume queries and verifies the results.
	testAssistantSetVolumeQuery(ctx, tconn, s)
	testAssistantIncreaseVolumeQuery(ctx, tconn, s)
}

// testAssistantTimeQuery verifies the correctness of the Assistant time response by parsing
// the fallback string to a time.Time object and comparing it with the current UTC time.
func testAssistantTimeQuery(ctx context.Context, tconn *chrome.Conn, s *testing.State) {
	response, err := assistant.SendTextQuery(ctx, tconn, "what time is it in UTC?")
	if err != nil {
		s.Error("Failed to get Assistant time response: ", err)
		return
	}

	if len(response.Fallback) == 0 {
		s.Error("No htmlFallback field found in the response")
		return
	}

	now := time.Now().UTC()
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

// testAssistantSetVolumeQuery tests setting volume action via Assistant.
func testAssistantSetVolumeQuery(ctx context.Context, tconn *chrome.Conn, s *testing.State) {
	const testVolume = 25
	_, err := assistant.SendTextQuery(ctx, tconn, fmt.Sprintf("set volume to %d.", testVolume))
	if err != nil {
		s.Errorf("Failed to set volume to %d via Assistant: %v", testVolume, err)
		return
	}

	v, err := getActiveNodeVolume(ctx)
	if err != nil {
		s.Error("Failed to get volume: ", err)
		return
	}

	if v != uint64(testVolume) {
		s.Errorf("System volume %d doesn't match the test volume %d", v, testVolume)
		return
	}
}

// testAssistantIncreaseVolumeQuery tests increasing volume action via Assistant.
func testAssistantIncreaseVolumeQuery(ctx context.Context, tconn *chrome.Conn, s *testing.State) {
	baseVolume, err := getActiveNodeVolume(ctx)
	if err != nil {
		s.Error("Failed to get volume: ", err)
		return
	}

	_, err = assistant.SendTextQuery(ctx, tconn, "turn up volume.")
	if err != nil {
		s.Error("Failed to increase volume via Assistant: ", err)
		return
	}
	v, err := getActiveNodeVolume(ctx)
	if err != nil {
		s.Error("Failed to get volume: ", err)
		return
	}

	if v <= baseVolume {
		s.Error("System volume doesn't increase: current - %d, base - %d", v, baseVolume)
		return
	}
}

// getActiveNodeVolume returns the current active node volume, ranging from 0 to 100.
func getActiveNodeVolume(ctx context.Context) (uint64, error) {
	cras, err := audio.NewCras(ctx)
	if err != nil {
		return 0, err
	}
	nodes, err := cras.GetNodes(ctx)
	if err != nil {
		return 0, err
	}

	var v uint64 = 101
	for _, n := range nodes {
		if n.Active && (!n.IsInput) {
			v = n.NodeVolume
			break
		}
	}
	if v > 100 {
		return 0, errors.New("cannot find active node volume from nodes")
	}

	return v, nil
}
