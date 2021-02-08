// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

#include <stdio.h>
#include <stdlib.h>
#include <sys/mman.h>
#include <unistd.h>

typedef struct AllocStackNode_ {
  struct AllocStackNode_ *prev;
  char *buffer;
} AllocStackNode;

static AllocStackNode *alloc_stack = NULL;
static size_t alloc_size = 0;
static size_t alloc_count = 0;

void allocNode(size_t bytes, size_t page_last_random, size_t page_size,
               FILE *r) {
  AllocStackNode *node = malloc(sizeof(AllocStackNode));
  if (node == NULL) {
    fprintf(stderr,
            "Failed to malloc AllocStackNode after allocating %zu nodes "
            "containing %zu bytes\n",
            alloc_count, alloc_size);
    exit(EXIT_FAILURE);
  }
  node->buffer = mmap(NULL, bytes, PROT_READ | PROT_WRITE,
                      MAP_PRIVATE | MAP_ANONYMOUS, -1, 0);
  if (node == MAP_FAILED) {
    perror("mmap failed, ");
    fprintf(stderr,
            "failed to mmap %zu byte buffer after allocating %zu buffers "
            "containing %zu bytes\n",
            bytes, alloc_count, alloc_size);
    exit(EXIT_FAILURE);
  }

  if (page_last_random > 0) {
    // Fill in random bytes to get the desired compressibility ratio.
    for (size_t page_index = 0; page_index < (bytes / page_size);
         page_index++) {
      char *page = &node->buffer[page_index * page_size];
      if (1 != fread(page, page_last_random, 1, r)) {
        perror("reading random bytes failed\n");
        exit(EXIT_FAILURE);
      }
      // NB: we don't need to initialize the compressible part of the page to
      // zero because it will already be zeros.
    }
  } else {
    // No random bytes needed, but we need to touch the pages to make them
    // resident.
    for (size_t page_index = 0; page_index < (bytes / page_size);
         page_index++) {
      char *page = &node->buffer[page_index * page_size];
      page[0] = 1;
      // NB: I don't want to have to worry about a sufficiently smart
      // compiler eliding an unused read or 0 write, so just write a 1.
    }
  }

  node->prev = alloc_stack;
  alloc_stack = node;
}

long get_oom_score_adj() {
  long score;
  FILE *oom_score_adj = fopen("/proc/self/oom_score_adj", "r");
  if (NULL == oom_score_adj) {
    perror("failed to open /proc/self/oom_score_adj\n");
    exit(EXIT_FAILURE);
  }
  if (1 != fscanf(oom_score_adj, "%ld", &score)) {
    perror("failed to read score from /proc/self/oom_score_adj\n");
    exit(EXIT_FAILURE);
  }
  if (0 != fclose(oom_score_adj)) {
    perror("failed to close /proc/self/oom_score_adj\n");
    exit(EXIT_FAILURE);
  }
  return score;
}

void set_oom_score_adj(long score) {
  FILE *oom_score_adj = fopen("/proc/self/oom_score_adj", "w");
  if (NULL == oom_score_adj) {
    perror("failed to open /proc/self/oom_score_adj\n");
    exit(EXIT_FAILURE);
  }
  if (0 > fprintf(oom_score_adj, "%ld", score)) {
    perror("failed to write score to /proc/self/oom_score_adj\n");
    exit(EXIT_FAILURE);
  }
  if (0 != fclose(oom_score_adj)) {
    perror("failed to close /proc/self/oom_score_adj\n");
    exit(EXIT_FAILURE);
  }
  long new_score = get_oom_score_adj();
  if (score != new_score) {
    fprintf(stderr, "failed to set oom_score_adj to %ld, got %ld", score,
            new_score);
    exit(EXIT_FAILURE);
  }
}

int main(int argc, char *argv[]) {
  if (argc != 5) {
    fprintf(stderr,
            "Usage: %s oom_score_alloc oom_score_done alloc_mib ratio\n",
            argv[0]);
    fprintf(stderr,
            "  oom_score_alloc - oom_score_adj to set while allocating\n");
    fprintf(stderr,
            "  oom_score_done - oom_score_adj to set after allocating\n");
    fprintf(stderr, "  alloc_mib - number of MiB to allocate\n");
    fprintf(stderr,
            "  ratio - how incompressible allocated memory is. 0.0 is very\n");
    fprintf(stderr, "          compressible, 1.0 is very incompressible\n\n");
    fprintf(stderr, "Allocates memory and then waits for input.\n");
    exit(EXIT_FAILURE);
  }

  long page_size = sysconf(_SC_PAGE_SIZE);
  if (page_size < 0) {
    perror("failed to get page size\n");
    exit(EXIT_FAILURE);
  }
  long oom_score_alloc = strtol(argv[1], NULL, 10);
  long oom_score_done = strtol(argv[2], NULL, 10);
  long alloc_mib = strtol(argv[3], NULL, 10);
  double ratio = strtod(argv[4], NULL);
  size_t page_last_random = (size_t)((double)page_size * ratio);

  FILE *r = fopen("/dev/urandom", "r");
  if (NULL == r) {
    perror("failed to open /dev/urandom\n");
    exit(EXIT_FAILURE);
  }

  // Allocate.
  set_oom_score_adj(oom_score_alloc);
  printf("allocating %ld 1MiB buffers, page_last_random = %zu\n", alloc_mib,
         page_last_random);
  for (long i = 0; i < alloc_mib; i++) {
    allocNode(1048576, page_last_random, page_size, r);
  }
  printf("done\n");
  fflush(stdout);

  if (0 != fclose(r)) {
    perror("failed to close /dev/urandom\n");
    exit(EXIT_FAILURE);
  }
  set_oom_score_adj(oom_score_done);

  // Wait forever.
  if (0 > pause()) {
    perror("failed to pause\n");
    exit(EXIT_FAILURE);
  }
  return 0;
}
