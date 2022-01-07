#!/usr/bin/env python3
# -*- coding: utf-8 -*-
# Copyright 2022 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""
Compiles C++ files into shared objects and adds them to an APK that has
already added its Java sources.
"""

import argparse
import pathlib
import subprocess
import sys

def compile_object_files(clang_bin):
  """Compile the .o files of each .cpp file.

  Args:
    clang_bin: Path to clang bin dir.
  """

  cmd = [
    clang_bin + "/clang++",
  ]
  subprocess.run(cmd, check=True)

def compile_so_file(clang_bin):
  """Compile a shared object files from a group of other files

  Args:
    clang_bin: Path to clang bin dir.

  Returns the so file path that was built.
  """
  pass

def package_so_object_into_apk(so_file, apk):
  """Package shared object file into an APK.

  Args:
    so_file: The file path for the input .so file.
    apk: The file path for the existing APK file.
  """
  pass

def get_parser():
  """Returns options parser."""
  parser = argparse.ArgumentParser()
  parser.add_argument('--clang-bin',
                      help='Clang binary directory.',
                      type=pathlib.Path)
  parser.add_argument('--unsigned-apk',
                      help='Path to the APK where java sources have been packaged into.',
                      type=pathlib.Path)
  parser.add_argument('files', nargs='+',
                      help='C++ source files.',
                      type=pathlib.Path)
  return parser

def main():
  args = sys.argv[1:]
  parser = get_parser()
  options = parser.parse(args)
  compile_object_files(options.clang_bin)
  compile_so_file(options.clang_bin)

if __name__ == '__main__':
  main()
