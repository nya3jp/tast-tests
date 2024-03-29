/*
 * Copyright 2017 The ChromiumOS Authors
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

/* Required for clock_adjtime(3). */
#define _GNU_SOURCE

#include <errno.h>
#include <string.h>
#include <time.h>

#include <sys/time.h>
#include <sys/timex.h>

/* This program is expected to run under android alt-syscall. */
int main(void) {
  struct timex buf;
  int ret;

  /* Test read operation. Should succeed (i.e. not returning -1). */
  memset(&buf, 0, sizeof(buf));
  ret = clock_adjtime(CLOCK_REALTIME, &buf);
  if (ret == -1)
    return 1;

  /*
   * Test a write operation. Under android alt-syscall, should fail with
   * EPERM.
   */
  buf.modes = ADJ_MAXERROR;
  ret = clock_adjtime(CLOCK_REALTIME, &buf);
  if (ret != -1 || errno != EPERM)
    return 3;

  /* Passed successfully */
  return 0;
}
