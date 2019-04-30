// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

#include <errno.h>
#include <sched.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>
#include <unistd.h>

// Amount of time to spin before returning.
const int kSpinNs = 1e8;  // 100 ms

// Number of recursive calls to make.
const int kRecursionDepth = 8;

// Returns the monotonically-increasing time as nanoseconds since the epoch.
int64_t GetTimeNs() {
  struct timespec ts = {0};
  if (clock_gettime(CLOCK_MONOTONIC, &ts) != 0) {
    fprintf(stderr, "clock_gettime: %s", strerror(errno));
    exit(1);
  }
  return 1e9 * ts.tv_sec + ts.tv_nsec;
}

// Calls itself n times. The innermost call spins for a short period of time
// before returning.
__attribute__((noinline)) void Recurse(int n) {
  if (n > 0) {
    Recurse(n-1);
    return;
  }

  const int64_t start = GetTimeNs();
  while (1) {
    if (GetTimeNs() - start >= kSpinNs) {
      break;
    }
    sched_yield();
  }
}

int main() {
  Recurse(kRecursionDepth);
  return 0;
}
