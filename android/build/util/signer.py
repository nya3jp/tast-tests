#!/usr/bin/env python
# -*- coding: utf-8 -*-
# Copyright 2020 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Add classes.dex into APK and sign it.
"""

import argparse
import os
import pathlib
import shutil
import subprocess
import sys


def sign(apk, output, key, cert, android_sdk_build_tools):
  """Create new APK signed with key and cert.

  Args:
    apk: Path to unsigned APK.
    output: Path to signed APK.
    key: Path to private key file.
    cert: Path to certificate chain.
    android_sdk_build_tools: Path to the Android SDK build tools.
  """
  os.makedirs(output.parent, exist_ok=True)
  shutil.copy(apk, output)
  cmd = [
      android_sdk_build_tools/'apksigner',
      "sign",
      "--key",
      key,
      "--cert",
      cert,
      output
  ]
  subprocess.run(cmd, check=True)


def get_parser():
  """Return option parser."""
  parser = argparse.ArgumentParser()
  parser.add_argument('unsigned_apk', help='unsigned apk file',
                      type=pathlib.Path)
  parser.add_argument('--output', help='output apk.', type=pathlib.Path,
                      required=True)
  parser.add_argument('--key', help='private key', type=pathlib.Path,
                      required=True)
  parser.add_argument('--cert', help='certificate chain', type=pathlib.Path,
                      required=True)
  parser.add_argument('--build-tools-dir',
                      help='Path to the Android SDK build tools directory.',
                      type=pathlib.Path)
  return parser


def main():
  argv = sys.argv[1:]
  parser = get_parser()
  options = parser.parse_args(argv)
  sign(options.unsigned_apk, options.output, options.key,
       options.cert, options.build_tools_dir)


if __name__ == '__main__':
  main()
