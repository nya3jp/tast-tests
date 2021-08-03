#!/usr/bin/env python3
# Copyright 2021 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""
Used to generate subtitles files for the given set of tast test results.

Usage:
  ./generate_subtitles.py [--chroot=<chroot dir>] [--timestamp=latest] [--open]

  chroot: The root chromiumos directory.
  timestamp: The timestamp corresponding to the tast file name (eg. 20210803-155906).
    Defaults to latest
  open: If provided, opens the video files after adding subtitles.

Example:

  ./generate_subtitles.py --chroot=~/chromiumos --open
"""

import argparse
import os
import re
import sys
import subprocess

from datetime import datetime, timedelta

_TEST_RESULTS_DIR = '/tmp/tast/results'
_LOG_LINE = re.compile(r'(\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d.\d{6})Z\s(?:\[\d\d:\d\d:\d\d.\d\d\d\] )?(.*)')

_SUBTITLE_HEADER = """
[Script Info]
ScriptType: v4.00+
Collisions: Normal
PlayResY: 600
PlayDepth: 0
Timer: 100,0000
Video Aspect Ratio: 0
Video Zoom: 6
Video Position: 0

[V4+ Styles]
Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding
Style: DefaultVCD, Arial,{fontsize},&H00B4FCFC,&H00B4FCFC,&H00000008,&H80000008,-1,0,0,0,100,100,0.00,0.00,1,1.00,2.00,1,30,30,30,0

[Events]
Format: Start, End, Style, Text
"""

def _parse_args():
  parser = argparse.ArgumentParser()
  parser.add_argument('--chroot', help='If outside the chroot, the root chromiumos directory')
  parser.add_argument(
      '--timestamp', default='latest',
      help='The timestamp used in the test results')
  parser.add_argument(
      '--duration', type=float, default=2,
      help='number of seconds to show each log message for')
  parser.add_argument(
      '--font-size', type=int, default=20,
      help='The size of the subtitle font')
  parser.add_argument(
      '--open', help='Whether to open the video after encoding subtitles', action='store_true')
  return parser.parse_args()

def _open_file(filename):
    if sys.platform == 'win32':
        os.startfile(filename)
    else:
        opener = 'open' if sys.platform == 'darwin' else 'xdg-open'
        print(opener, filename)
        subprocess.run([opener, filename])

# .ass files require 2 decimal places after the seconds.
def _format_ass_timestamp(ts: timedelta):
  hundredths = ts.microseconds // 10000
  ts -= timedelta(microseconds=ts.microseconds)
  return f'{ts}.{hundredths:02}'

def _add_subtitles(test_dir: str, args):
  start_ts = None
  lines = []
  with open(os.path.join(test_dir, "log.txt")) as f:
    for line in f:
      match = _LOG_LINE.match(line.rstrip())
      if match is not None:
        timestamp, msg = match.groups()
        timestamp = datetime.strptime(timestamp, '%Y-%m-%dT%H:%M:%S.%f')
        if start_ts is not None:
          lines.append((timestamp - start_ts, msg))
        elif msg == "Started screen recording":
          start_ts = timestamp
      elif start_ts is not None:
        ts, old_line = lines[-1]
        lines[-1] = (ts, old_line + "\\N" + line.rstrip())

  videos = [os.path.join(test_dir, f) for f in os.listdir(test_dir) if f.endswith(".webm")]
  for video in videos:
    with open(video[:-5] + ".ass", "w") as f:
      f.write(_SUBTITLE_HEADER.format(fontsize=args.font_size))

      for ts, msg in lines:
        msg = msg.replace('\t', '  ')
        end_ts = ts + timedelta(seconds=args.duration)
        f.write(f"Dialogue: {_format_ass_timestamp(ts)},{_format_ass_timestamp(end_ts)},DefaultVCD,{msg}\n")

    if args.open:
      _open_file(video)

def main():
  args = _parse_args()

  test_results_dir =_TEST_RESULTS_DIR if args.chroot is None \
    else os.path.join(os.path.expanduser(args.chroot), 'chroot', _TEST_RESULTS_DIR.lstrip('/'))
  test_results_dir = os.path.join(test_results_dir, args.timestamp, 'tests')
  if not os.path.isdir(test_results_dir):
    raise Exception(f'Invalid path: {test_results_dir}')

  for test in os.listdir(test_results_dir):
    _add_subtitles(os.path.join(test_results_dir, test), args)


if __name__ == '__main__':
  main()
