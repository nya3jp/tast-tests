#!/usr/bin/env python3
# -*- coding: utf-8 -*-
# Copyright 2019 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Compile java files and create classes.dex files from compiled files.
"""

import os
import subprocess
import sys
import argparse

import find


def javac(files, android_sdk, output):
  """Run javac and create class files."""
  os.makedirs(output, exist_ok=True)
  cmd = [
      'javac',
      '-classpath',
      android_sdk,
      '-d',
      output,
  ]
  cmd += files
  return subprocess.run(cmd)


def d8(files, android_sdk, output, build_tools_dir=""):
  """Run d8 and create classes.dex."""
  os.makedirs(output, exist_ok=True)
  cmd = [
      os.path.join(build_tools_dir, 'd8'),
      '--lib',
      android_sdk,
      '--output',
      output
  ]
  cmd += files
  return subprocess.run(cmd)


def get_parser():
  """Return options parser."""
  parser = argparse.ArgumentParser()
  parser.add_argument('--android-sdk',
                      help='Path to the Android SDK directory.',
                      required=True)
  parser.add_argument('--output',
                      help='Output path.')
  parser.add_argument('--class-dir',
                      help='Class directory path.')
  parser.add_argument('--build-tools-dir',
                      help='Path to the Android SDK build tools directory.')
  parser.add_argument('files', nargs='+',
                      help='Resouces directories.')
  return parser


def main():
  args = sys.argv[1:]
  parser = get_parser()
  options = parser.parse_args(args)
  exit_status = javac(options.files,
                      output=options.class_dir,
                      android_sdk=options.android_sdk)
  if not exit_status:
    return exit_status
  files = list(find.find_files(options.class_dir, "*.class"))
  exit_status = d8(files, output=options.output,
                   android_sdk=options.android_sdk,
                   build_tools_dir=options.build_tools_dir)
  if not exit_status:
    return exit_status
  return 0


if __name__ == '__main__':
    sys.exit(main())
