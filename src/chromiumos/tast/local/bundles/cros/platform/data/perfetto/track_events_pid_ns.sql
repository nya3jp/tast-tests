-- This query outputs the PID of track events of the perfetto_simple_producer process.
-- Sample output in TSV:
-- category	name	tid
-- perfetto_simple_producer	Trial1	6838
-- perfetto_simple_producer	Trial2	6838
-- perfetto_simple_producer	Trial3	6838
create view track_event_slices as
  select slice.category, slice.name, thread.tid
    from slice join thread_track on slice.track_id = thread_track.id join thread on thread_track.utid = thread.utid
    where slice.category='perfetto_simple_producer';

-- This query produces the PID of kernel scheduling slices of the perfetto_simple_producer process.
-- Sample output in TSV:
-- tid	name
-- 6838	perfetto_simple
-- 6838	perfetto_simple
-- 6838	perfetto_simple
-- 6838	perfetto_simple
-- 6838	perfetto_simple
-- 6838	perfetto_simple
-- 6838	perfetto_simple
create view ftrace_slices as
  select thread.tid, thread.name from sched_slice join thread on sched_slice.utid = thread.utid where thread.name = 'perfetto_simple';

-- Join the above 2 views with matching tid.
-- If PID namespace is supported, the track events should use the root-level PID and the join should produce a valid row.
-- Sample output in TSV:
-- name	tid
-- Trial1	6838
select track_event_slices.name, track_event_slices.tid from track_event_slices join ftrace_slices on track_event_slices.tid = ftrace_slices.tid limit 1;
