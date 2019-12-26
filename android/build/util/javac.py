#!/usr/bin/env python3
# -*- coding: utf-8 -*-
# Copyright 2019 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Compile java files and create classes.dex files from compiled files.
"""

import os
import shutil
import subprocess
import sys
import argparse

import find


def javac(sources, android_sdk_platform, output_dir):
  """Run javac and create class files.

  Args:
    sources: List of paths to java sources.
    android_sdk_platform: Path to the Android SDK Platform
    output_dir: Directory in which to generate .class files.
  """
  os.makedirs(output_dir, exist_ok=True)
  cmd = [
      'javac',
      '-XDskipDuplicateBridges=true',
      '-XDstringConcat=inline',
      '-source', '1.8',
      '-target', '1.8',
      '-d',
      output_dir,
      '-classpath',
      android_sdk_platform,
  ]
  cmd += sources
  subprocess.run(cmd, check=True)


def d8(classes, android_sdk_platform, output_dir, android_sdk_build_tools=""):
  """Run d8 and create classes.dex.

  Args:
    classes: List of paths to .class files.
    android_sdk_platform: Path to the Android SDK Platform.
    output_dir: Directory in which to generate .dex files.
    android_sdk_build_tools: Path to the Android SDK build tools.
  """
  os.makedirs(output_dir, exist_ok=True)
  cmd = [
      os.path.join(android_sdk_build_tools, 'd8'),
      '--lib',
      android_sdk_platform,
      '--output',
      output_dir
  ]
  cmd += classes
  subprocess.run(cmd, check=True)


def add_dexes(resource_apk, dexes, output):
  """Create new APK added classes.dex into.

  Args:
    resource_apk: Path to the APK including only resource.
    dexes: List of paths to classes.dex.
    output: Path to the generated APK including classes.dex.
  """
  dirname = os.path.dirname(output)
  os.makedirs(dirname, exist_ok=True)
  shutil.copy(resource_apk, output)
  cmd = [
      "zip",
      "-uj",
      output,
  ]
  cmd += dexes
  subprocess.run(cmd, check=True)


def get_parser():
  """Return options parser."""
  parser = argparse.ArgumentParser()
  parser.add_argument('--android-sdk-platform',
                      help='Android SDK Platform path.',
                      required=True)
  parser.add_argument('--output',
                      help='Path to output APK.')
  parser.add_argument('--dex-dir',
                      help='Directory in which to generate classes.dex')
  parser.add_argument('--class-dir',
                      help='Directory in which to generate .class files')
  parser.add_argument('--android-sdk-build-tools',
                      help='Android SDK build tools path.')
  parser.add_argument('--resource-apk',
                      help='Path to the APK including only resource')
  parser.add_argument('files', nargs='+',
                      help='Resources directories.')
  return parser


def main():
  args = sys.argv[1:]
  parser = get_parser()
  options = parser.parse_args(args)
  javac(options.files, output_dir=options.class_dir,
        android_sdk_platform=options.android_sdk_platform)
  files = list(find.find_files(options.class_dir, "*.class"))
  d8(files, output_dir=options.dex_dir,
     android_sdk_platform=options.android_sdk_platform,
     android_sdk_build_tools=options.android_sdk_build_tools)
  dexes = list(find.find_files(options.dex_dir, "*.dex"))
  add_dexes(options.resource_apk, dexes, options.output)


if __name__ == '__main__':
    main()
