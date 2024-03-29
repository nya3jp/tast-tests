/*
 * Copyright 2015 The ChromiumOS Authors
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

#include <fcntl.h>
#include <stdlib.h>
#include <unistd.h>

#include <sys/mman.h>
#include <sys/stat.h>
#include <sys/types.h>

int main(void) {
  char *buf;
  int fd, ret;
  unsigned int i;

  fd = open("/dev/zero", O_RDONLY);
  if (fd < 0)
    return 1;

  buf = mmap(NULL, 4096, PROT_READ, MAP_PRIVATE, fd, 0);
  if (buf == (char *)-1)
    return 2;

  for (i = 0; i < 4096; i++) {
    if (buf[i] != 0)
      return 3;
  }

  ret = munmap(buf, 4096);
  if (ret < 0)
    return 4;

  ret = close(fd);
  if (ret < 0)
    return 5;

  return 0;
}
