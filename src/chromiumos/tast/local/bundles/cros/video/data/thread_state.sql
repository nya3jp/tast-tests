select ts, dur, state, thread.tid, thread.name --, thread.is_main_thread
from thread_state left join thread using(utid)
where utid IN (select utid
      from thread
      where upid = (
            select upid
            from process
            where cmdline like '%--type=gpu-process%'))
order by ts
