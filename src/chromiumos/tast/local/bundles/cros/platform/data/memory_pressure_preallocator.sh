#! /bin/bash -e

# Copyright 2019 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# Usage: snarfer.sh <size_mb> <pagefile>
# Snarfs <size_mb> megabytes of RAM and sits there until killed.
# The RAM is filled with data from <pagefile>, repeated.
#
# This uses multiple "dd" processes with a large output buffer (obs=...), and
# with an output stream which doesn't accept input, so that the output buffer
# fills up and sits there.  "Dd" allocates the output buffer in normal anonymous
# memory, so it can be swapped out.  The data is a repetition of a page that
# compresses with the desired ratio.  (This may become a problem if the
# compressor also does deduplication.)
#
# The current datafile was created by trial and error using "seq 1000 | od -c |
# dd bs=4096 count=1".  It compresses to 1525 bytes with lzop (0.37 ratio) and
# 1579 bytes with lz4 (0.39).  This is similar to the compression ratio observed
# for the page set used by this test.
#
# I also tried creating the data file in the "obvious" way: dd from /dev/zero
# and /dev/random in the right proportions.  Unfortunately this seems to trigger
# a bug in the ARM kernel, where such page is apparently compressed correctly,
# but zram ends up using a full page of storage for each compressed page.


size=$1
pagefile=$2
if [ -z "${size}" -o -z "${pagefile}" ]; then
  echo "usage: $0 <size_mb> <pagefile>"
  exit 1
fi

# Sanity check.  This will also fail (uglyly) if $size is too large to be parsed
# into a valid integer value.
if [ "${size}" -ge 100000 ]; then
  echo "size_mb must be less than 100000"
fi

TEMPDIR=$(mktemp -d)
trap "rm -rf ${TEMPDIR}" EXIT ERR INT
DATAFILE="${TEMPDIR}/data"

# Create a larger data file for speed.
echo > $DATAFILE
for i in $(seq 128); do
  cat ${pagefile} >> $DATAFILE
done

bloater () {
  while true; do
    cat $DATAFILE
  done | dd obs="$1M" status=none | sleep infinity
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

# Wait forever for a kill signal, which is automatically propagated to the
# bloater processes.
sleep infinity
