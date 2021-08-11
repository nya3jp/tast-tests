// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

#include <time.h>
#include <stdio.h>
#include <stdint.h>

#define rdtscll(val) do { \
  uint32_t a,d; \
  asm volatile("rdtsc" : "=a" (a), "=d" (d)); \
  (val) = ((uint64_t)a) | (((uint64_t)d)<<32); \
} while(0)

int main() {
  struct timespec clock_boottime;
  struct timespec clock_monotonic;
  uint64_t tsc;

  clock_gettime(CLOCK_BOOTTIME, &clock_boottime);
  clock_gettime(CLOCK_MONOTONIC, &clock_monotonic);
  rdtscll(tsc);

  puts("{");
  printf("  \"CLOCK_BOOTTIME\": { \"tv_sec\": %10ld, \"tv_nsec\": %10ld },\n",
    clock_boottime.tv_sec, clock_boottime.tv_nsec);
  printf("  \"CLOCK_MONOTONIC\": { \"tv_sec\": %10ld, \"tv_nsec\": %10ld },\n",
    clock_monotonic.tv_sec, clock_monotonic.tv_nsec);
  printf("  \"TSC\": %lu\n", tsc);
  puts("}");
  return 0;
}
