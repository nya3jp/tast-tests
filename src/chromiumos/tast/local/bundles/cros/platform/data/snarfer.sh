#! /bin/bash -e

# Copyright 2019 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# Usage: snarf <size_mb> <compression_ratio>
# Snarfs <size_mb> megabytes of RAM and sits there until killed.
# The RAM is filled with data that compresses with the given
# compression ratio.

size=$1
ratio=$2
if [ -z "$size" -o -z "$ratio" ]; then
  echo "usage: $0 <size_mb> <compression_ratio>"
  exit 1
fi

random_size=$(awk "BEGIN {printf(\"%d\", 4096 * $ratio)}")
zero_size=$((4096 - random_size))

TEMPDIR=$(mktemp -d)
trap "rm -rf ${TEMPDIR}" EXIT ERR INT
PAGEFILE="${TEMPDIR}/page"
DATAFILE="${TEMPDIR}/data"

echo > ${PAGEFILE}
dd status=none if=/dev/zero bs=$zero_size count=1 >> ${PAGEFILE}
dd status=none if=/dev/urandom bs=$random_size count=1 >> ${PAGEFILE}

echo > $DATAFILE
for i in $(seq 128); do
  cat ${PAGEFILE} >> $DATAFILE
done

bloater () {
  while true; do
    cat $DATAFILE
  done | dd obs="$1"M status=none | sleep infinity
}

# Max bloater size in MB.
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
