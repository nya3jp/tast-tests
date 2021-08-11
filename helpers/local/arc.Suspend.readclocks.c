// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

#include <time.h>
#include <stdio.h>

int main() {
  struct timespec clock_boottime;
  struct timespec clock_monotonic;

  clock_gettime(CLOCK_BOOTTIME, &clock_boottime);
  clock_gettime(CLOCK_MONOTONIC, &clock_monotonic);

  puts("{");
  printf("  \"CLOCK_BOOTTIME\": { \"tv_sec\": %10ld, \"tv_nsec\": %10ld },\n",
    clock_boottime.tv_sec, clock_boottime.tv_nsec);
  printf("  \"CLOCK_MONOTONIC\": { \"tv_sec\": %10ld, \"tv_nsec\": %10ld }\n",
    clock_monotonic.tv_sec, clock_monotonic.tv_nsec);
  puts("}");
  return 0;
}
