-- Here is an example of battery counters in the trace data:
-- select id, ts, name, value from counters where name like 'batt.%'
-- id    ts                 name                                              value
-- ----- ------------------ ------------------------------------------------- ---------------------
--     0      2828870663872 batt.hid-0018:27C6:0E52.0001-battery.capacity_pct              0.000000
--     1      2828870818377 batt.sbs-12-000b.charge_uah                              5450000.000000
--     2      2828870818377 batt.sbs-12-000b.capacity_pct                                100.000000
--     3      2828870818377 batt.sbs-12-000b.current_ua                                    0.000000
--
-- The counter name is in the form of batt.BATTERY_NAME.COUNTER_NAME.
-- BATTERY_NAME is optional and will be present only when the system has multiple batteries.
-- Note that there are 2 reported batteries
--   hid-0018:27C6:0E52.0001-battery - the stylus battery. It only reports capacity.
--   sbs-12-000b - the main battery. It reports capacity, charge and current counters.
--
-- This query file outputs the counter names and average counter values.
-- Sample output:
-- "name","avg(value)"
-- "batt.sbs-12-000b.capacity_pct","100.000000"
-- "batt.sbs-12-000b.charge_uah","5450000.000000"
-- "batt.sbs-12-000b.current_ua","0.000000"
--
select name, avg(value) from counters
 where name like 'batt.%' and
       -- Filter out stylus battery counters as we are only interested in the
       -- main battery.
       name not like 'batt.hid-%'
 group by name
