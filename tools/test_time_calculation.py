#!/usr/bin/env python3
# Copyright 2022 The ChromiumOS Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""
Parse test result logs to obtain the average and max time from various
executions.

Usage:
  ./test_time_calculation.py log_file [--test_name=package.test.parameter]
  log_file: The name of the file resulting from the repeated executions of the
    test. It can be obtained by recording the output of running the
    repeat-test.sh script
  test_name: Name of the test to evaluate. If provided, only the resulting time
  from the test that matches this value is evaluated. This is useful if more
  than one parameters of a test have been executed at the same time.

Example:
  $repeat-test.sh -s -r 10 -- DUT policy.AllowDinosaurEasterEgg > tests_log.txt
  ./test_time_calculation.py tests_log.txt --test_name=policy.AllowDinosaurEasterEgg
"""

import argparse

def _parse_args():
  parser = argparse.ArgumentParser()
  parser.add_argument('log_file', help='Name of the file with the full results')
  parser.add_argument(
    '--test_name', default='',
    help='Specify the name of the test to parse')
  return parser.parse_args()

def _time_from_log_line(line):
  words = line.split()
  # Time should be immediately after "in" and there is only one occurence of
  # this word in the corresponding line.
  test_time = words[words.index("in")+1]
  # Convert the time to seconds. Time in the logs has the following format:
  # 0.00s or 0m0.00s. We assume no test will take an hour or more, so parsing
  # minutes and seconds should be enough.
  # There is always an s at the end, so we can simply remove it first.
  test_time = test_time[:-1]
  # If the test took longer than a minute, there will be an m, we try to split
  # the time by it and convert the time to seconds.
  test_time_m_s = test_time.split("m")
  test_time_val = 0.0
  time_id = 0
  if len(test_time_m_s) > 1:
    # The first element of the array correspond to minutes, we convert it to
    # seconds.
    test_time_val = 60*float(test_time_m_s[time_id])
    # The second element corresponds to the seconds, reassign the index.
    time_id = 1

  # Add seconds to time.
  test_time_val += float(test_time_m_s[time_id])

  return test_time_val

def main():
  args = _parse_args()

  n_runs = 0
  total_time = 0
  max_time = 0

  with open(args.log_file) as log_file:
      for line in log_file:
          if "Completed test "+args.test_name in line:
            print(line[:-1])
            test_time = _time_from_log_line(line)
            n_runs += 1
            total_time += test_time
            if (test_time > max_time):
              max_time = test_time
            print("Test run " + str(n_runs) + ":  "+ str(test_time)+"s")

  avg_time = total_time/n_runs
  print()
  print("Resulting time from " + str(n_runs) +" runs. Total: "+str(total_time)+" Avg: "+str(avg_time)+"s. Max: "+str(max_time)+"s")

if __name__ == '__main__':
  main()