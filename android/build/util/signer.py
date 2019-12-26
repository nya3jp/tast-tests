#!/usr/bin/env python
# -*- coding: utf-8 -*-
# Copyright 2020 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Add classes.dex into APK and sign it.
"""

import argparse
import os
import shutil
import subprocess
import sys


def sign(apk, output, key, cert, android_sdk_build_tools=""):
  """Create new APK signed with key and cert.

  Args:
    apk: Path to unsigned APK.
    output: Path to signed APK.
    key: Path to private key file.
    cert: Path to certificate chain.
    android_sdk_build_tools: Path to the Android SDK build tools.
  """
  dirname, _ = os.path.split(output)
  os.makedirs(dirname, exist_ok=True)
  shutil.copy(apk, output)
  cmd = [
      os.path.join(android_sdk_build_tools, "apksigner"),
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
  parser.add_argument('--dex', help='dex file.')
  parser.add_argument('--unsigned-apk', help='unsigned apk file',
                      required=True)
  parser.add_argument('--output', help='output apk.', required=True)
  parser.add_argument('--key', help='private key', required=True)
  parser.add_argument('--cert', help='certificate chain', required=True)
  parser.add_argument('--build-tools-dir',
                      help='Path to the Android SDK build tools directory.')
  return parser


def main():
  argv = sys.argv[1:]
  parser = get_parser()
  options = parser.parse_args(argv)
  sign(options.unsigned_apk, options.output, options.key,
       options.cert, options.build_tools_dir)


if __name__ == '__main__':
  main()
