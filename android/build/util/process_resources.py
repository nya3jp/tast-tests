#!/usr/bin/env python3
# -*- coding: utf-8 -*-
# Copyright 2019 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Compile resources and link them.
"""

import os
import subprocess
import sys
import argparse

import find


def compile_resources(output, files, build_tools_dir=""):
  """Run aapt2 compile and create .flat files."""
  os.makedirs(output, exist_ok=True)
  cmd = [os.path.join(build_tools_dir, "aapt2"), 'compile', '-o', output]
  cmd += files
  return subprocess.run(cmd)


def link_resources(files, output, rjava, android_sdk, manifest,
                   target_sdk_version=None, build_tools_dir=""):
  """Run aapt2 link and create R.java and APK."""
  cmd = [
      os.path.join(build_tools_dir, "aapt2"),
      'link',
      '-o',
      output,
      '--java',
      rjava,
      '--manifest',
      manifest,
      '-I',
      android_sdk,
  ]
  if target_sdk_version:
    cmd += ['--target-sdk-version', target_sdk_version]
  cmd += files

  return subprocess.run(cmd)


def get_parser():
  """Return option parser."""
  parser = argparse.ArgumentParser()
  parser.add_argument('--android-sdk',
                      help='Path to the Android SDK directory.',
                      required=True)
  parser.add_argument('--build-tools-dir',
                      help='Path to the Android SDK build tools directory.')
  parser.add_argument('--manifest', help='AndroidManifest.xml path.',
                      required=True)
  parser.add_argument('--Rjava',
                      help='Directory in which to generate R.java.',
                      required=True)
  parser.add_argument('--target-sdk-version',
                      help='Default target SDK version to use for '
                           'AndroidManifest.xml.')
  parser.add_argument('--output', help='Output path.')
  parser.add_argument('--compile-dir', help='Flat files path.')
  parser.add_argument('files', nargs='+', help='Resouces directories.')
  return parser


def main():
  args = sys.argv[1:]
  parser = get_parser()
  options = parser.parse_args(args)
  exit_status = compile_resources(options.compile_dir, options.files,
                                  options.build_tools_dir)
  if not exit_status:
    return exit_status
  flat_files = list(find.find_files([options.compile_dir], '*.flat'))
  exit_status = link_resources(flat_files,
                               output=options.output,
                               rjava=options.Rjava,
                               android_sdk=options.android_sdk,
                               manifest=options.manifest,
                               target_sdk_version=options.target_sdk_version,
                               build_tools_dir=options.build_tools_dir)
  if not exit_status:
    return exit_status
  return 0


if __name__ == '__main__':
  sys.exit(main())
