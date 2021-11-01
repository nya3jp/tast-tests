#!/usr/bin/env python3
# Copyright 2021 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

import argparse
import dataclasses
import json
import os
import pathlib
import re
import subprocess

TAST_PKG_CACHE = pathlib.Path("/tmp/tast_list_packages_cache.json")
TAST_CMD_CACHE = pathlib.Path("/tmp/tast_last_debugger_cmd")
TEST_FORMAT = re.compile(r"[a-z0-9]+\.([A-Z][a-zA-Z0-9]*)(?:\.[a-z]+)?")
TEST_PREFIX= re.compile(".*?/platform/tast-tests/src/")
BUNDLE_FORMAT = re.compile("tast-tests/src/chromiumos/tast/(local|remote)/bundles")

parser = argparse.ArgumentParser()
parser.add_argument("--dut", required=True, help="IP of dut")
parser.add_argument("--current-file", required=True, help="The file currently open in vscode")

args = parser.parse_args()

if not TAST_PKG_CACHE.exists():
  print("Creating a file -> test mapping now. Please be patient (first time setup).")
  with open(TAST_PKG_CACHE, "w") as f:
    subprocess.run(["tast", "list", "-json", "dut"], stdout=f, check=True)

# These directories are symlinks. Since we care about the path and not the
# contents, we'll need to resolve it (and the path doesn't work, since it
# was generated outside the chroot).
args.current_file = re.sub("tast-tests/(local|remote)_tests",
                           r"tast-tests/src/chromiumos/tast/\1/bundles/cros",
                           args.current_file)

def save_debugger_command():
  """Determines the test & test params to debug, & saves the params to disk."""
  bundle_match = BUNDLE_FORMAT.search(args.current_file)
  path = re.sub(TEST_PREFIX, "", args.current_file)
  expected_pkg, expected_test_name = os.path.split(path)
  if bundle_match is None or not expected_test_name.endswith(".go"):
    print("The currently open file in vscode is not a test file. Debugging your most recently debugged test.")
    return

  bundle = bundle_match.group(1)
  # my_test.go -> MyTest. Tast literally has a lint for this, so we won't get
  # funny business where they have a different name.
  expected_test_name = re.sub("_([a-z])",
                              lambda s: s.group(1).upper(),
                              "_" + expected_test_name[:-3])

  with open(TAST_PKG_CACHE) as f:
    tests = json.load(f)
  matches = []
  for test in tests:
    name = test["name"]
    if test["pkg"] == expected_pkg:
      test_name = TEST_FORMAT.match(name).group(1)
      if test_name == expected_test_name:
        matches.append(name)
  matches = sorted(matches)

  if not matches:
    print(f"The currently open file in vscode appears not to be a test file. If that is incorrect, try running 'rm {TAST_PKG_CACHE}'. Debugging your most recently debugged test.")
    return

  if len(matches) == 1:
    test = matches[0]
    print(f"Detected test {test} open in vscode, running it.")
  else:
    print("Detected multiple possible options for the test file currently open in vscode. Please select the one you intend to run.")
    for i, test in enumerate(matches):
      print(f"[{i}] {test}")
    test = matches[int(input("Enter a number corresponding to a test: "))]

  extra_args = input("Enter any additional arguments you want to provide to tast (eg. -var=keepState=true): ")
  with open(TAST_CMD_CACHE, "w") as f:
    f.write(f"{bundle}\n{test}\n{extra_args}\n")

save_debugger_command()

if not TAST_CMD_CACHE.exists():
  print("Couldn't work out what test to run, and no test was previously run. Can't do anything, exiting")
  exit(1)
with open(TAST_CMD_CACHE) as f:
  bundle, test_name, extra_args = [f.strip() for f in f.readlines()]
cmd = f"tast run -attachdebugger={bundle}:2345 {args.dut} {extra_args} {test_name}"
print(f"Running command: {cmd}")
os.execlp("sh", "sh", "-c", cmd)
