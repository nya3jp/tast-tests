#!/usr/bin/env python3
# -*- coding: utf-8 -*-
# Copyright 2020 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Compile resources, merge them and make APK.
"""

import os
import shutil
import subprocess
import sys
import argparse

import find


def compile_resources(output_dir, resources, android_sdk_build_tools=""):
  """Run aapt2 compile and create .flat files.

  Args:
    output_dir: Directory in which to generate .flat files.
    resources: Paths to the resources to be compiled.
    android_sdk_build_tools: Path to the android SDK build tools.
  """
  os.makedirs(output_dir, exist_ok=True)
  cmd = [
      os.path.join(android_sdk_build_tools, "aapt2"),
      'compile',
      '-o',
      output_dir]
  cmd += resources
  subprocess.run(cmd, check=True)


def link_resources(files, output_apk, rjava_dir, android_sdk_platform, manifest,
                   target_sdk_version=None, min_sdk_version=None, overlay=None,
                   android_sdk_build_tools=""):
  """Run aapt2 link and create R.java and APK.

  Args:
    files: Paths to the flatted resources to be merged.
    output_apk: Path to the generated APK.
    rjava_dir: Directory in which to generate R.java.
    android_sdk_platform: Path to the Android SDK Platform.
    manifest: Path to the AndroidManifest.xml.
    target_sdk_version: Default target SDK version to use for
      AndroidManifest.xml.
    min_sdk_version: Default minimum SDK version to use for AndroidManifest.xml.
    android_sdk_build_tools: Path to the Android SDK build tools.
  """
  cmd = [
      os.path.join(android_sdk_build_tools, "aapt2"),
      'link',
      '-o',
      output_apk,
      '--java',
      rjava_dir,
      '--manifest',
      manifest,
      '-I',
      android_sdk_platform,
      '--no-static-lib-packages',
      '--auto-add-overlay',
  ]
  if overlay:
    cmd += ['-R', f'@{overlay}']
  if target_sdk_version:
    cmd += ['--target-sdk-version', target_sdk_version]
  if min_sdk_version:
    cmd += ['--min-sdk-version', min_sdk_version]
  cmd += files
  subprocess.run(cmd, check=True)


def move_rjava(rjava_dir):
  """Move R.java directly under rjava_dir.

  Args:
    rjava_dir: Directory in which to move R.java.
  """
  rjava = list(find.find_files([rjava_dir], 'R.java'))
  if len(rjava) == 0:
    raise Exception("R.java not found.")
  if len(rjava) > 1:
    raise Exception("There are multiple R.java. Run `gn clean` and try again.")
  shutil.move(rjava[0], os.path.join(rjava_dir, 'R.java'))


def get_parser():
  """Return option parser."""
  parser = argparse.ArgumentParser()
  parser.add_argument('--android-sdk-platform',
                      help='Android SDK Platform path.',
                      required=True)
  parser.add_argument('--android-sdk-build-tools',
                      help='Android SDK build tools path.')
  parser.add_argument('--manifest', help='Path to the AndroidManifest.xml.',
                      required=True)
  parser.add_argument('--Rjava-dir',
                      help='Directory in which to generate R.java.',
                      required=True)
  parser.add_argument('--target-sdk-version',
                      help='Default target SDK version to use for '
                           'AndroidManifest.xml.')
  parser.add_argument('--min-sdk-version',
                      help='Default minimum SDK version to use for'
                           'AndroidManifest.xml')
  parser.add_argument('--overlay',
                      help='overlay')
  parser.add_argument('--output', help='Path to output APK.')
  parser.add_argument('--compile-dir', help='Path to the compiled resources.')
  parser.add_argument('files', nargs='+', help='Resource directories.')
  return parser


def main():
  args = sys.argv[1:]
  parser = get_parser()
  options = parser.parse_args(args)
  compile_resources(output_dir=options.compile_dir,
                    resources=options.files,
                    android_sdk_build_tools=options.android_sdk_build_tools)

  flat_files = list(find.find_files([options.compile_dir], '*.flat'))
  link_resources(flat_files,
                 output_apk=options.output,
                 rjava_dir=options.Rjava_dir,
                 android_sdk_platform=options.android_sdk_platform,
                 manifest=options.manifest,
                 target_sdk_version=options.target_sdk_version,
                 min_sdk_version=options.min_sdk_version,
                 overlay=options.overlay,
                 android_sdk_build_tools=options.android_sdk_build_tools)
  move_rjava(options.Rjava_dir)


if __name__ == '__main__':
  main()
