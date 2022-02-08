-- Sample output of battery counters from the query:
-- select id, ts, name, value from counters where name like 'batt.%'
-- id                   ts                   name                 value
-- -------------------- -------------------- -------------------- --------------------
--                    0     1802253909102978 batt.charge_uah            3005000.000000
--                    1     1802253909102978 batt.capacity_pct              100.000000
--                    2     1802253909102978 batt.current_ua                  0.000000
--                    3     1802254000349096 batt.charge_uah            3005000.000000
--                    4     1802254000349096 batt.capacity_pct              100.000000
--                    5     1802254000349096 batt.current_ua                  0.000000
--
-- This query file outputs average capacity percent, coulomb counter (in uAh) and current (in uA).
-- Sample output:
-- "capacity_percent","charge_uah","current_ua"
-- 100.000000,3005000.000000,18454.545455
select * from (
  (select avg(value) as capacity_percent from counters where name='batt.capacity_pct'),
  (select avg(value) as charge_uah from counters where name='batt.charge_uah'),
  (select avg(value) as current_ua from counters where name='batt.current_ua')
);
