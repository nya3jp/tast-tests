#! /bin/bash -e

# Copyright 2019 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# Usage: snarf <size_mb> <pagefile>
# Snarfs <size_mb> megabytes of RAM and sits there until killed.
# The RAM is filled with data from <pagefile>, repeated.

size=$1
pagefile=$2
if [ -z "${size}" -o -z "${pagefile}" ]; then
  echo "usage: $0 <size_mb> <pagefile>"
  exit 1
fi

TEMPDIR=$(mktemp -d)
trap "rm -rf ${TEMPDIR}" EXIT ERR INT
DATAFILE="${TEMPDIR}/data"

echo > $DATAFILE
for i in $(seq 128); do
  cat ${pagefile} >> $DATAFILE
done

bloater () {
  while true; do
    cat $DATAFILE
  done | dd obs="$1"M status=none | sleep infinity
}

# Max bloater size in MB.  This must be less than 2G.
MAXSIZE=1024
# Starts as many bloater processes as required.  Multiple processes are needed
# because dd has a max buffer size of 2GB - 1.
while true; do
  if [ $size -gt $MAXSIZE ]; then
    bloater $MAXSIZE &
    size=$(( size - MAXSIZE ))
  else
    bloater $size &
    break
  fi
done

# Wait for a kill signal, which is automatically propagated to the bloater
# processes.
sleep infinity
