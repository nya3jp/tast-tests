#!/usr/bin/env python
# -*- coding: utf-8 -*-
# Copyright 2020 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Finds files in directories.
"""

import fnmatch
import argparse
import os
import pathlib
import sys


def find_files(paths, pattern):
  """Iterate files matched with pattern in path."""
  if not isinstance(paths, list):
    paths = [paths]
  for path in paths:
    for file in path.rglob(pattern):
      if file.is_file():
        yield file.absolute()


def main():
  argv = sys.argv[1:]
  parser = argparse.ArgumentParser()
  parser.add_argument('--pattern', default='*', help='File pattern to match.')
  parser.add_argument('path', nargs='+', type=pathlib.Path)
  options = parser.parse_args(argv)
  for file in find_files(options.path, options.pattern):
    print(file)


if __name__ == '__main__':
  main()