// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

#include <errno.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/socket.h>
#include <sys/un.h>
#include <unistd.h>

int recbomb(int n);
void PrepareBelow(int argc, char *argv[]);
extern int DefeatTailOptimizationForCrasher();
int DefeatTailOptimizationForBomb() {
  return 0;
}

int main(int argc, char *argv[]) {
  PrepareBelow(argc, argv);
  return recbomb(16) + DefeatTailOptimizationForCrasher();
}

bool SendPid(const char *socket_path);

// Prepare for doing the crash, but do it below main so that main's
// line numbers remain stable.
void PrepareBelow(int argc, char *argv[]) {
  fprintf(stderr, "pid=%jd\n", (intmax_t) getpid());
  if (argc == 2 && strcmp(argv[1], "--nocrash") == 0) {
    fprintf(stderr, "Doing normal exit\n");
    exit(0);
  }
  if (argc == 3 && strcmp(argv[1], "--sendpid") == 0) {
    if (!SendPid(argv[2]))
      exit(1);
  }
  fprintf(stderr, "Crashing as requested.\n");
}

// This is the same macro as defined in base/posix/eintr_wrapper.h.
// Imported here to avoid depending on the large libchrome package.
#if defined(NDEBUG)

#define HANDLE_EINTR(x) ({ \
  decltype(x) eintr_wrapper_result; \
  do { \
    eintr_wrapper_result = (x); \
  } while (eintr_wrapper_result == -1 && errno == EINTR); \
  eintr_wrapper_result; \
})

#else

#define HANDLE_EINTR(x) ({ \
  int eintr_wrapper_counter = 0; \
  decltype(x) eintr_wrapper_result; \
  do { \
    eintr_wrapper_result = (x); \
  } while (eintr_wrapper_result == -1 && errno == EINTR && \
           eintr_wrapper_counter++ < 100); \
  eintr_wrapper_result; \
})

#endif  // NDEBUG

// Used when the crasher runs in a different PID namespace than the test. A PID
// sent over a Unix domain socket to a process in a different PID namespace is
// converted to that PID namespace.
bool SendPid(const char *socket_path) {
  struct Socket {
    Socket(): fd(socket(AF_UNIX, SOCK_DGRAM, 0)) {}
    ~Socket() { if (fd != -1) close(fd); }
    int fd;
  } sock;

  if (sock.fd == -1) {
    fprintf(stderr,"socket() failed: %s\n", strerror(errno));
    return false;
  }

  sockaddr_un address = { AF_UNIX };
  strncpy(address.sun_path, socket_path, sizeof(address.sun_path) - 1);
  sockaddr *address_ptr = reinterpret_cast<sockaddr *>(&address);
  if (HANDLE_EINTR(connect(sock.fd, address_ptr, sizeof(address))) == -1) {
    fprintf(stderr, "connect() failed: %s\n", strerror(errno));
    return false;
  }

  char zero = '\0';
  iovec data = { &zero, 1 };
  msghdr msg = { NULL, 0, &data, 1 };

  if (HANDLE_EINTR(sendmsg(sock.fd, &msg, 0)) == -1) {
    fprintf(stderr, "sendmsg() failed: %s\n", strerror(errno));
    return false;
  }

  return true;
}
