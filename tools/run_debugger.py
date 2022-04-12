#!/usr/bin/env python3
# Copyright 2021 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

import argparse
import dataclasses
import json
import os
import pathlib
import pickle
import re
import subprocess
from typing import Optional

TAST_PKG_CACHE = pathlib.Path("/tmp/tast_list_packages_cache.json")
TAST_CMD_CACHE = pathlib.Path("/tmp/tast_last_debugger_cmd.pickle")
TEST_FORMAT = re.compile(r"[a-z0-9]+\.([A-Z][a-zA-Z0-9]*)(?:\.[a-z]+)?")
TEST_PREFIX= re.compile(".*?/platform/tast-tests/src/")
BUNDLE_FORMAT = re.compile("tast-tests/src/chromiumos/tast/(local|remote)/bundles")

parser = argparse.ArgumentParser()
parser.add_argument("--dut", required=True, help="IP of dut")
parser.add_argument("--current-file", required=True, help="The file currently open in vscode")
parser.add_argument("--fast-build-tast", action="store_const", dest="tast_binary",
                    default="tast", const="~/go/bin/tast",
                    help="Use the version of tast built by fast_build.sh")
parser.add_argument("--debug", action="store_const", dest="debug", const=True,
                    help="Run tast test and wait for a debugger to attach")
parser.add_argument("--no-debug", action="store_const", dest="debug", const=False,
                    help="Run tast without waiting for a debugger to attach")

args = parser.parse_args()

# Because I've handed out the instructions for debugging already, debugging has
# to be true by default (changing this would break existing use cases).
if args.debug is None:
  args.debug = True

if not TAST_PKG_CACHE.exists():
  print("Creating a file -> test mapping now. Please be patient (first time setup).")
  with open(TAST_PKG_CACHE, "w") as f:
    subprocess.run(["tast", "list", "-json", args.dut], stdout=f, check=True)

# These directories are symlinks. Since we care about the path and not the
# contents, we'll need to resolve it (and the path doesn't work, since it
# was generated outside the chroot).
args.current_file = re.sub("tast-tests/(local|remote)_tests",
                           r"tast-tests/src/chromiumos/tast/\1/bundles/cros",
                           args.current_file)

@dataclasses.dataclass
class DebuggerCommand:
  bundle: str
  test: str
  extra_args: str

def load_last_debugger_command() -> Optional[DebuggerCommand]:
  if TAST_CMD_CACHE.exists():
    with open(TAST_CMD_CACHE, "rb") as f:
      return pickle.load(f)

def save_debugger_command() -> Optional[DebuggerCommand]:
  """Determines the test & test params to debug, & saves the params to disk."""
  bundle_match = BUNDLE_FORMAT.search(args.current_file)
  path = re.sub(TEST_PREFIX, "", args.current_file)
  expected_pkg, expected_test_name = os.path.split(path)
  if bundle_match is None or not expected_test_name.endswith(".go"):
    print("The currently open file in vscode is not a test file. "
          "Debugging your most recently debugged test.")
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
    if test["pkg"] == expected_pkg:
      name = test["name"]
      test_name = TEST_FORMAT.match(name).group(1)
      # Initializations don't match properly without converting to lower.
      # eg. service_grpc has test name ServiceGRPC, but we would normally
      # expect ServiceGrpc.
      if test_name.lower() == expected_test_name.lower():
        matches.append(name)
  matches = sorted(matches)

  if not matches:
    print("The currently open file in vscode appears not to be a test file. "
          f"If that is incorrect, try running 'rm {TAST_PKG_CACHE}'. "
          "Debugging your most recently debugged test.")
    return

  # Same file as last time, so don't re-prompt for the subtest / extra args.
  last_cmd = load_last_debugger_command()
  if last_cmd is not None and last_cmd.test in matches:
    return last_cmd

  if len(matches) == 1:
    test = matches[0]
    print(f"Detected test {test} open in vscode, running it.")
  else:
    print("Detected multiple possible options for the test file currently "
          "open in vscode. Please select the one you intend to run.")
    for i, test in enumerate(matches):
      print(f"[{i}] {test}")
    test = matches[int(input("Enter a number corresponding to a test: "))]

  extra_args = input("Enter any additional arguments you want to provide to "
                     "tast (eg. -var=keepState=true): ")
  cmd = DebuggerCommand(bundle=bundle, test=test, extra_args=extra_args)
  with open(TAST_CMD_CACHE, "wb") as f:
    pickle.dump(cmd, f)
  return cmd

cmd = save_debugger_command() or load_last_debugger_command()
if cmd is None:
  print("Couldn't work out what test to run, and no test was previously run. "
        "Can't do anything, exiting")
  exit(1)

debug_args = f"-attachdebugger={cmd.bundle}:2345" if args.debug else ""
run = f"{args.tast_binary} run {debug_args} {cmd.extra_args} {args.dut} {cmd.test}"
print(f"Running command: {run}")
os.execlp("sh", "sh", "-c", run)
