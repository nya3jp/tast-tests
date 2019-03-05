// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/assistant"
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
	res, err := assistant.SendTextQuery(ctx, tconn, "what time is it in UTC?")
	if err != nil {
		s.Fatal("Failed to get Assistant response: ", err)
	}
	if err := verifyTimeResponse(ctx, res); err != nil {
		s.Fatal("Failed to verify the time response: ", err)
	}
}

// verifyTimeResponse parses the Assistant time response and compares it with the current UTC
// time to verify the correctness.
func verifyTimeResponse(ctx context.Context, response assistant.Response) error {
	fallback := response.Fallback
	if len(fallback) == 0 {
		return errors.New("No htmlfallback field found in the response")
	}

	now := time.Now().UTC()
	assistantTime, err := parseTimeNearNow(fallback, now)
	if err != nil {
		return err
	}

	// Two time results may not exactly match due to the response latency or clock drift, so we
	// instead check if |assistantTime| falls within an acceptable time range around |now|.
	tolerance := time.Minute
	earliest := now.Add(-tolerance)
	latest := now.Add(tolerance)
	if assistantTime.Before(earliest) || assistantTime.After(latest) {
		return errors.New("Time results didn't matched")
	}
	return nil
}

// parseTimeNearNow extracts the numeric components in the fallback string to construct a time
// object based on now time. For this particular query, a typical fallback text will have format:
// "It is (HH/H):MM AM/PM in the Coordinated Universal Time zone".
func parseTimeNearNow(fallback string, now time.Time) (time.Time, error) {
	re := regexp.MustCompile(`((\d{1,2}:\d{1,2}|\d{1,2}) [AaPp][Mm])`)
	timeStr := re.FindString(fallback)
	if len(timeStr) == 0 {
		return time.Time{}, errors.New("Fallback doesn't contain any well-formatted time results")
	}

	// Parses hours and minutes from the time string with format (HH/H):MM AM/PM.
	t := strings.Split(strings.Split(timeStr, " ")[0], ":")
	hrs, err := strconv.Atoi(t[0])
	if err != nil {
		return time.Time{}, err
	}
	var min int
	if len(t) > 1 {
		min, err = strconv.Atoi(t[1])
		if err != nil {
			return time.Time{}, err
		}
	}

	// Converts the assistant time to 24-hour clock to be consistent with the system time.
	if strings.Contains(timeStr, "AM") || strings.Contains(timeStr, "am") {
		if hrs == 12 {
			hrs -= 12
		}
	} else if strings.Contains(timeStr, "PM") || strings.Contains(timeStr, "pm") {
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
