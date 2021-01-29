// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package gpucuj

import (
	"testing"

	"android.googlesource.com/platform/external/perfetto/protos/perfetto/trace"
	"github.com/golang/protobuf/proto"
)

func TestTraceSimple(t *testing.T) {
	s := `
packet: <
	clock_snapshot: <
	  clocks: <
      clock_id: 6
      timestamp: 2491717086881
    >
    clocks: <
      clock_id: 64
      timestamp: 2491718272
      unit_multiplier_ns: 1000
    >
    clocks: <
      clock_id: 65
      timestamp: 2491718272
      is_incremental: true
      unit_multiplier_ns: 1000
    >
  >
  trusted_uid: 0
  trusted_packet_sequence_id: 3
  sequence_flags: 1
  trace_packet_defaults: <
    timestamp_clock_id: 65
    track_event_defaults: <
      track_uuid: 9964562508143186970
      extra_counter_track_uuids: 9964562512438154266
    >
  >
  previous_packet_dropped: true
>
packet: <
  timestamp: 0
  track_descriptor: <
    uuid: 9964562508143186970
    parent_uuid: 9964562508143196990
    thread: <
      pid: 7111
      tid: 12068
      thread_name: "ThreadPoolForegroundWorker"
    >
    chrome_thread: <
      thread_type: THREAD_POOL_FG_WORKER
    >
  >
  trusted_uid: 0
  trusted_packet_sequence_id: 3
>
packet: <
  timestamp: 7
  track_event: <
    category_iids: 1
    name_iid: 1
    type: TYPE_SLICE_BEGIN
    extra_counter_values: 8
    task_execution: <
      posted_from_iid: 1
    >
  >
  trusted_uid: 0
  trusted_packet_sequence_id: 3
  interned_data: <
    event_categories: <
      iid: 1
      name: "toplevel"
    >
    event_names: <
      iid: 1
      name: "ThreadPool_RunTask"
    >
  >
  sequence_flags: 2
>`

	tr := &trace.Trace{}
	if err := proto.UnmarshalText(s, tr); err != nil {
		t.Fatal("could not unmarshal proto data: ", err)
	}

	ta := newTraceAnalyzer(tr)
	count := 0
	err := ta.parseTrackEvents(func(ts uint64, trackName string, sd *seqData, p *trace.TracePacket, flowID uint64, newFlow bool) error {
		count++
		// 7 us offset from clock ID 65, then normalized to boottime builtin clock.
		if ts != 2491717086881+7*1000 {
			t.Errorf("got timestamp %d, want 2491718279", ts)
		}
		// Default UUID
		if sd.uuid != 9964562508143186970 {
			t.Errorf("got UUID %d, want 9964562508143186970", sd.uuid)
		}
		// ThreadPoolForegroundWorker track name
		if trackName != "ThreadPoolForegroundWorker" {
			t.Errorf("got track name %s, want ThreadPoolForegroundWorker", trackName)
		}
		// No flow events
		if flowID != 0 {
			t.Errorf("got flow ID %d, want 0", flowID)
		}
		return nil
	})
	if err != nil {
		t.Error("got error: ", err)
	}
	if count != 1 {
		t.Errorf("got %d packets, want 1", count)
	}
}
