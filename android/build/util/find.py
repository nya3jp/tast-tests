#!/usr/bin/env python
# -*- coding: utf-8 -*-
# Copyright 2020 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Finds files in directories.
"""

import argparse
import pathlib
import sys


def find_files(path, pattern):
  """Iterate files matched with pattern in path."""
  for file in path.rglob(pattern):
    if file.is_file():
      yield file.absolute()


def main():
  argv = sys.argv[1:]
  parser = argparse.ArgumentParser()
  parser.add_argument('--pattern', default='*', help='File pattern to match.')
  parser.add_argument('paths', nargs='+', type=pathlib.Path)
  options = parser.parse_args(argv)
  for path in options.paths:
    for file in find_files(path, options.pattern):
      print(file)


if __name__ == '__main__':
  main()