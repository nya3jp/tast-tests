#!/bin/bash
#
# Copyright 2020 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.
#
# This script assists developers identify test flakiness and reproduce
# errors in flaky tests.
#
# This script should be executed from inside cros_sdk.
#

err() {
  echo "$@" >&2
}

print_usage() {
  err "Usage: $0 [-s] -r <Repetitions> -- TAST_RUN_ARGS"
  err "Repeatedly execute a test to find and debug test flakes."
  err "  -s stops test execution on the first failure of the test."
  err ""
  err "Run 'tast run --help' for documentation on TAST_RUN_ARGS."
  err ""
  err "Examples:"
  err "To run the test 'example.DBus' 100 times and stop on the first failure:"
  err "  $0 -s -r 100 -- 172.16.243.1 example.DBus"
  err "To run the test 'example.Internal' from bundle 'crosint' 10 times:"
  err "  $0 -r 10 -- -buildbundle=crosint 172.16.243.1 example.Internal"
}

usage_error=""
repetitions=""
stop_on_error=""

while getopts 'r:s' flag; do
  case "${flag}" in
    r) repetitions="${OPTARG}" ;;
    s) stop_on_error="true" ;;
    # An unrecognised option was used.
    *) usage_error="true" ;;
  esac
done

readonly repetitions
readonly stop_on_error

# Capture the tast arguments after the --
shift "$((OPTIND-1))"
readonly tast_args=("$@")

if [[ -z "${repetitions}" ]]
then
  err "Missing number of test repetitions."
  print_usage
  exit 1
fi

if [[ -n "${usage_error}" ]]
then
  err "Invalid option used."
  print_usage
  exit 1
fi

for i in $(seq 1 "${repetitions}")
do
  echo "* Starting execution ${i}"
  echo "* Executing \"tast run -failfortests ${tast_args[*]}\""
  tast run -failfortests "${tast_args[@]}"
  result=$?
  if [[ "${result}" -ne 0 ]]
  then
    err "* Test exited with error ${result}."
    if [[ -n "${stop_on_error}" ]]
    then
      echo "* Stopping tests due to first error (-s option was specified)."
      break
    else
      cat <<EOF
* If you tried to stop the tests using Ctrl+C, you should press Ctrl+C again
* in quick succession to stop the tests.
EOF
    fi

    # It is possible the tests failed because the user pressed Ctrl+C.
    # Give the user a second to press Ctrl+C a second time, to break out
    # of the tests completely.
    if ! sleep 1
    then
      echo "* Stopping tests (user aborted)."
      break
    fi
  else
    echo "* Test exited successfully."
  fi
done

