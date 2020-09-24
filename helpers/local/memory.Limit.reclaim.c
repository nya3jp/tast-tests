/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

#include <limits.h>
#include <stdbool.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#define SMALL_ZONE_THRESHOLD 1024

ssize_t close_to_oom() {
  FILE *f = fopen("/proc/zoneinfo", "r");
  char *buffer = NULL;
  size_t buffer_len = 0;
  int zone_node;
  char zone_name[64];
  int zone_free;
  int zone_min;
  int zone_low;
  ssize_t result = SSIZE_MAX;

  if (f == NULL) {
    perror("failed to open /proc/zoneinfo");
    exit(EXIT_FAILURE);
  }

  while (-1 != getline(&buffer, &buffer_len, f)) {
    if (2 == sscanf(buffer, "Node %d, zone %63s", &zone_node, zone_name)) {
      zone_free = -1;
      zone_min = -1;
    } else if (1 == sscanf(buffer, " pages free %d", &zone_free)) {
    } else if (1 == sscanf(buffer, " min %d", &zone_min)) {
    } else if (1 == sscanf(buffer, " low %d", &zone_low)) {
      if (zone_min > SMALL_ZONE_THRESHOLD) {
        if (-1 == zone_free || -1 == zone_min) {
          fprintf(stderr, "Node %d, zone %s missing free or min", zone_node,
                  zone_name);
          exit(EXIT_FAILURE);
        }
        ssize_t zone_diff =
            (ssize_t)zone_free - ((ssize_t)zone_min + (ssize_t)zone_low) / 2;
        zone_diff *= 4096;
        if (zone_diff < result) {
          result = zone_diff;
        }
      }
    }
  }

  free(buffer);
  fclose(f);
  return result;
}

int main(int argc, char *argv[]) {
  char *buffer = NULL;
  size_t buffer_len = 0;
  ssize_t line_len;

  while (-1 != (line_len = getline(&buffer, &buffer_len, stdin))) {
    if (line_len == 1) {
      // Includes NULL, so exit if length is 1.
      break;
    }
    if (0 == strncmp(buffer, "distance\n", line_len)) {
      fprintf(stdout, "%zd\n", close_to_oom());
      fflush(stdout);
    } else {
      fprintf(stderr, "unsupported operation\n");
      return 1;
    }
  }
  free(buffer);
  return 0;
}
