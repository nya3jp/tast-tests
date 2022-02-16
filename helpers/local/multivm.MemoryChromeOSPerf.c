// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

#define GNU_SOURCE

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/mman.h>
#include <unistd.h>

static void verify_oom_score_adj(const char *score) {
  FILE *file = fopen("/proc/self/oom_score_adj", "r");
  if (NULL == file) {
    perror("failed to open /proc/self/oom_score_adj\n");
    exit(EXIT_FAILURE);
  }
  char buffer[16] = {0};
  size_t read = fread(buffer, 1, sizeof(buffer) - 1, file);
  if (0 == read) {
    fprintf(stderr, "Failed to read /proc/self/oom_score_adj\n");
    exit(EXIT_FAILURE);
  }
  // Remove the trailing \n.
  buffer[read - 1] = '\0';
  if (0 != strcmp(buffer, score)) {
    fprintf(stderr,
            "Failed to verify oom_score_adj, expected\"%s\", got \"%s\"\n",
            score, buffer);
    exit(EXIT_FAILURE);
  }
  if (0 != fclose(file)) {
    fprintf(stderr, "Failed to close /proc/self/oom_score_adj\n");
    exit(EXIT_FAILURE);
  }
}

static void set_oom_score_adj(const char *score) {
  FILE *file = fopen("/proc/self/oom_score_adj", "w");
  if (NULL == file) {
    perror("failed to open /proc/self/oom_score_adj\n");
    exit(EXIT_FAILURE);
  }
  if (1 != fwrite(score, strlen(score), 1, file)) {
    fprintf(stderr, "Failed to write oom_score_adj\n");
    exit(EXIT_FAILURE);
  }
  if (0 != fclose(file)) {
    fprintf(stderr, "Failed to close /proc/self/oom_score_adj\n");
    exit(EXIT_FAILURE);
  }
  verify_oom_score_adj(score);
}

typedef struct alloc_stack_node_t_ {
  struct alloc_stack_node_t_ *prev;
  size_t bytes;
  char *buffer;
} alloc_stack_node_t;

static alloc_stack_node_t *alloc_stack = NULL;
static size_t alloc_size = 0;
static size_t alloc_count = 0;

static void alloc_node(FILE *r, size_t bytes, size_t page_last_random,
                       size_t page_size) {
  alloc_stack_node_t *node = malloc(sizeof(alloc_stack_node_t));
  if (node == NULL) {
    fprintf(stderr,
            "Failed to malloc alloc_stack_node_t after allocating %zu nodes "
            "containing %zu bytes\n",
            alloc_count, alloc_size);
    exit(EXIT_FAILURE);
  }
  node->bytes = bytes;
  node->buffer = mmap(NULL, bytes, PROT_READ | PROT_WRITE,
                      MAP_PRIVATE | MAP_ANONYMOUS, -1, 0);
  if (node->buffer == MAP_FAILED) {
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
      size_t bytes_left = bytes - page_index * page_size;
      size_t rand_bytes =
          page_last_random < bytes_left ? page_last_random : bytes_left;
      if (1 != fread(&node->buffer[page_index * page_size], rand_bytes, 1, r)) {
        fprintf(stderr,
                "failed to read random bytes, feof() := %d; ferror() := %d",
                feof(r), ferror(r));
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
  alloc_size += bytes;
  alloc_count++;
  node->prev = alloc_stack;
  alloc_stack = node;
}

static void free_node() {
    if (alloc_stack == NULL) {
      fprintf(stderr, "nothing to free\n");
      exit(EXIT_FAILURE);
    }
    alloc_stack_node_t *node = alloc_stack;
    alloc_stack = node->prev;
    if (0 != munmap(node->buffer, node->bytes)) {
      perror("free_node munmap failed\n");
      exit(EXIT_FAILURE);
    }
    alloc_size -= node->bytes;
    alloc_count--;
    free(node);
}

#define MIB 1048576

static void allocate_anon(FILE *r, long page_size) {
  size_t size;
  float ratio;
  if (2 != scanf("%zu %f", &size, &ratio)) {
    fprintf(stderr, "Failed to read anon arguments\n");
    exit(EXIT_FAILURE);
  }
  if (ratio < 0.0f || ratio > 1.0f) {
    fprintf(stderr, "Compression ratio should be between 0 and 1, got %f\n",
            ratio);
    exit(EXIT_FAILURE);
  }
  size_t size_remaining = size;
  size_t page_last_random = (int)((float)page_size * ratio);
  while (size_remaining > 0) {
    size_t node_size = size_remaining < MIB ? size_remaining : MIB;
    alloc_node(r, node_size, page_last_random, page_size);
    size_remaining -= node_size;
  }
  printf(
      "allocated %zu bytes of anonymous memory, total %zu bytes over %zu "
      "allocations\n",
      size, alloc_size, alloc_count);
}

static void free_anon() {
  size_t size;
  if (1 != scanf("%zu", &size)) {
    fprintf(stderr, "Failed to read free size\n");
    exit(EXIT_FAILURE);
  }
  if (size > alloc_size) {
    fprintf(stderr, "Can not free %zu bytes, only %zu bytes allocated\n",
            size, alloc_size);
    exit(EXIT_FAILURE);
  }
  size_t alloc_target = alloc_size - size;
  while (alloc_size > alloc_target) {
    free_node();
  }
}

static void alloc_repl() {
  FILE *r = fopen("/dev/urandom", "r");
  if (NULL == r) {
    perror("failed to open /dev/urandom\n");
    exit(EXIT_FAILURE);
  }

  long page_size = sysconf(_SC_PAGE_SIZE);
  if (page_size < 0) {
    perror("failed to get page size\n");
    exit(EXIT_FAILURE);
  }

  while (1) {
    char verb[16];
    if (1 != scanf("%15s", verb)) {
      fprintf(stderr, "Failed to read command\n");
      exit(EXIT_FAILURE);
    }
    if (0 == strcmp("anon", verb)) {
      allocate_anon(r, page_size);
    } else if (0 == strcmp("free", verb)) {
      free_anon();
    } else if (0 == strcmp("exit", verb)) {
      printf("exiting\n");
      return;
    } else {
      fprintf(stderr, "Unknown command \"%s\"\n", verb);
      exit(EXIT_FAILURE);
    }
    fflush(stdout);
  }
}

static void print_usage(const char *exe) {
  printf("%s <oom_score_adj>\n", exe);
  printf("  Starts an allocation REPL with the following commands:\n");
  printf("    anon <size> <ratio>\n");
  printf("      Allocate anonymous memory of <size> bytes, and compression\n");
  printf("      ratio <ratio> (e.g. 1.0 is not compressible, 0.5 compresses\n");
  printf("      to half size)\n");
  printf("    free <size>\n");
  printf("      Free memory until at least <size> bytes have been freed, or\n");
  printf("      there is no memory allocated left. Memory types are freed\n");
  printf("      in the reverse order they were allocated.\n");
  printf("    exit\n");
  printf("      Exit the program.\n");
  printf("\n");
  printf("Arguments:\n");
  printf("  oom_score_adj: int - Set the OOM score of the test program.\n");
  printf("\n");
}

int main(int argc, char *argv[]) {
  if (2 != argc) {
    print_usage(argv[0]);
    fprintf(stderr, "Expected 1 arg, got %d\n", argc - 1);
    return EXIT_FAILURE;
  }
  set_oom_score_adj(argv[1]);
  alloc_repl();
  return EXIT_SUCCESS;
}
