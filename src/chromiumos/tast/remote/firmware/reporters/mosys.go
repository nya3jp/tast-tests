// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package reporters

import (
	"context"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
)

// Event contains the contents of one line from `mosys eventlog list`.
type Event struct {
	Timestamp time.Time
	Message   string
	Index     int
}

// EventlogList returns the result of `mosys eventlog list`.
// The returned events are sorted from oldest to newest.
func (r *Reporter) EventlogList(ctx context.Context) ([]Event, error) {
	output, err := r.CommandOutputLines(ctx, "mosys", "eventlog", "list")
	if err != nil {
		return []Event{}, err
	}
	now, err := r.Now(ctx)
	if err != nil {
		return []Event{}, errors.Wrap(err, "getting current time")
	}
	const timeFmt = "2006-01-02 15:04:05"
	var events []Event
	// Expecting output similar to this one:
	//  140 | 2021-09-20 15:11:55 | EC Event | Key Pressed
	//  141 | 2021-09-20 15:13:30 | System boot | 45
	//  142 | 2021-09-20 15:13:30 | System Reset
	for _, line := range output {
		split := strings.SplitN(line, " | ", 3)
		if len(split) < 3 {
			return []Event{}, errors.Errorf("eventlog entry had fewer than 3 ' | ' delimiters: %q", line)
		}
		var timestamp time.Time
		// If the timestamp is missing, it is printed at 2000-00-00 00:00:00, but that is not a valid date and can't be parsed.
		if split[1] != "2000-00-00 00:00:00" {
			timestamp, err = time.Parse(timeFmt, split[1])
			if err != nil {
				return []Event{}, err
			}
		}
		if timestamp.After(now) {
			return []Event{}, errors.Errorf("event occurred later than current DUT time: now=%s; event=%s", now, line)
		}
		index, err := strconv.ParseInt(split[0], 10, 0)
		if err != nil {
			return []Event{}, errors.Errorf("failed to parse index %q", split[0])
		}
		events = append(events, Event{
			Timestamp: timestamp,
			Message:   split[2],
			Index:     int(index),
		})
	}
	return events, nil
}

// EventlogListAfter returns a list of events that occurred after a given index.
func (r *Reporter) EventlogListAfter(ctx context.Context, previousEvent Event) ([]Event, error) {
	events, err := r.EventlogList(ctx)
	if err != nil {
		return []Event{}, errors.Wrap(err, "reporting events")
	}
	// EventlogList reports events from oldest to newest.
	// Iterate through the events in reverse order to return only the newest ones.
	for i := len(events) - 1; i > 0; i-- {
		if events[i].Timestamp.Before(previousEvent.Timestamp) || (events[i].Timestamp.Equal(previousEvent.Timestamp) && events[i].Index <= previousEvent.Index) {
			return events[i+1:], nil
		}
	}
	return events, nil
}
