// Copyright 2012 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

#include <fcntl.h>
#include <stdlib.h>
#include <unistd.h>
#include <asm/unistd.h>

#define SIZE 1024

int main(int argc, char **argv) {
  char buf[SIZE];
  int fd = syscall(__NR_openat, AT_FDCWD, "/dev/zero", O_RDONLY);
  syscall(__NR_read, fd, buf, SIZE);
  syscall(__NR_close, fd);
  syscall(__NR_exit, 0);
}
