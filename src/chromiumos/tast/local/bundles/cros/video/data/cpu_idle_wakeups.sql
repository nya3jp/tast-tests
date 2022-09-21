select thread.utid, thread.name, process.name, count(*)
from counter as c
left join cpu_counter_track as t on c.track_id = t.id
left join sched on sched.id = (select id
     from sched
     where ts > c.ts and cpu = t.cpu
     order by ts desc
     limit 1)
left join thread using(utid)
left join process using(upid)
where t.name = 'cpuidle' and value = 4294967295
group by 1,2,3
order by 4 desc
