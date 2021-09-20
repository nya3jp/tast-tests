// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
// Simple program that calls brk. Used to verify crash_reporter's behavior on
// seccomp violations.

#include <stdlib.h>
#include <unistd.h>
#include <asm/unistd.h>

int main(int argc, char **argv) {
  syscall(__NR_brk, NULL);
  syscall(__NR_exit, 0);
}
