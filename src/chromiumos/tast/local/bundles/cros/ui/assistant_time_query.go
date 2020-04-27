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
		Func:         AssistantTimeQuery,
		Desc:         "Tests Assistant time query response",
		Contacts:     []string{"meilinw@chromium.org", "xiaohuic@chromium.org"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Pre:          chrome.LoggedIn(),
	})
}

func AssistantTimeQuery(ctx context.Context, s *testing.State) {
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

	s.Log("Sending time query to the Assistant")
	queryStatus, err := assistant.SendTextQuery(ctx, tconn, "what time is it in UTC?")
	if err != nil {
		s.Fatal("Failed to get Assistant time response: ", err)
	}

	s.Log("Verifying the time query response")
	fallback := queryStatus.QueryResponse.Fallback
	if fallback == "" {
		s.Fatal("No response sent back from Assistant")
	}

	now := time.Now().UTC()
	// Truncates the sec and nsec to be consistent with the format of assistant time response
	// and reduce the time error in interpretation.
	now = time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), 0, 0, now.Location())
	results, err := parseTimeNearNow(fallback, now)
	if err != nil {
		s.Fatal("Failed to parse Assistant time response: ", err)
	}

	// Two time results may not exactly match due to the response latency or clock drift, so
	// we instead check if any time in |results| falls within an acceptable time range around
	//|now|.
	tolerance := time.Minute
	earliest := now.Add(-tolerance)
	latest := now.Add(tolerance)
	for _, assistantTime := range results {
		s.Logf("Comparing Assistant time %v with the current time %v", assistantTime, now)
		// Both boundary values must be allowed given our interpretation rules. It is worth
		// mentioning that t.Before(t) and t.After(t) will both return false, so we check it
		// by excluding values fall beyond the range.
		if !(assistantTime.Before(earliest) || assistantTime.After(latest)) {
			return
		}
	}

	s.Fatal("Assistant time response doesn't match the current time")
}

// parseTimeNearNow extracts the numeric components in the fallback string to construct a time
// object based on now time. For this particular query, a typical fallback text will have format:
// "It is (HH/H):MM AM/PM in the Coordinated Universal Time zone." in English locale.
func parseTimeNearNow(fallback string, now time.Time) ([]time.Time, error) {
	re := regexp.MustCompile(`(\d{1,2})(:\d\d)? ([AaPp][Mm])?`)
	matches := re.FindStringSubmatch(fallback)
	if matches == nil {
		return nil, errors.Errorf("fallback %s doesn't contain any well-formatted time results", fallback)
	}

	// Parses hours and minutes from the time string.
	hrs, err := strconv.Atoi(matches[1])
	if err != nil {
		return nil, err
	}
	var min int
	if len(matches[2]) != 0 {
		min, err = strconv.Atoi(matches[2][1:])
		if err != nil {
			return nil, err
		}
	}

	results := make([]time.Time, 0, 2)
	if matches[3] != "" {
		// The time response returned from Assistant is in 12-hour AM/PM format, we then
		// convert it to use 24-hour clock to be consistent with the system time.
		if strings.EqualFold(matches[3], "AM") {
			if hrs == 12 {
				hrs -= 12
			}
		} else if strings.EqualFold(matches[3], "PM") {
			if hrs != 12 {
				hrs += 12
			}
		}
		results = append(results, interpretTimeNearNow(now, hrs, min))
	} else {
		// If there is no "AM" or "PM" substring found in the fallback string, then:
		// (1) The Assistant time is still in 12-hour clock, but with AM/PM displayed in a different
		// language. This could happen if the test device uses a non-English system locale, or
		// (2) The Assistant time is in 24-hour format, as we observed that both the 12-hour and
		// 24-hour notations are used in Assistant depending on which locale it is set to.
		if hrs > 12 || hrs == 0 {
			// Under this case we know Assistant is using 24-hour clock.
			results = append(results, time.Date(now.Year(), now.Month(), now.Day(), hrs, min, 0, 0, now.Location()))
		} else {
			// If hrs <= 12, Assistant might be using either time notation.
			results = append(results, interpretTimeNearNow(now, hrs, min), interpretTimeNearNow(now, (hrs+12)%24, min))
		}
	}

	return results, nil
}

// interpretTimeNearNow chooses the interpretation closest to the now time because the fallback
// string only contains HH:MM.
func interpretTimeNearNow(now time.Time, hrs, min int) time.Time {
	t := time.Date(now.Year(), now.Month(), now.Day(), hrs, min, 0, 0, now.Location())
	if diff := t.Sub(now); diff > 12*time.Hour {
		t.AddDate(0, 0, -1)
	} else if diff < -12*time.Hour {
		t.AddDate(0, 0, 1)
	}
	return t
}
