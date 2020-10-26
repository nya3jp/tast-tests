// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

#include <errno.h>
#include <fcntl.h>
#include <inttypes.h>
#include <signal.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>
#include <unistd.h>
#include <sys/time.h>

#define SIG SIGRTMIN

typedef struct {
    const char* name;
    clockid_t id;
} named_clock_t;

static int out_desc = STDOUT_FILENO; /* default is stdout, prog param */

// Make sure to keep this array in sync with `clock*` constants in the test
// hardware.VerifyRemoteSleep.
static named_clock_t clocks[] = {
    {"CLOCK_REALTIME", CLOCK_REALTIME},
    {"CLOCK_REALTIME_COARSE", CLOCK_REALTIME_COARSE},
    {"CLOCK_MONOTONIC", CLOCK_MONOTONIC},
    {"CLOCK_MONOTONIC_COARSE", CLOCK_MONOTONIC_COARSE},
    {"CLOCK_MONOTONIC_RAW", CLOCK_MONOTONIC_RAW},
    {"CLOCK_BOOTTIME", CLOCK_BOOTTIME},
    {"CLOCK_PROCESS_CPUTIME_ID", CLOCK_PROCESS_CPUTIME_ID},
    {"CLOCK_THREAD_CPUTIME_ID", CLOCK_THREAD_CPUTIME_ID}
};
static size_t clock_n = sizeof(clocks) / sizeof(named_clock_t);
static clockid_t clockid;

static void send_msg(const char *msg) {
    if(write(out_desc, msg, strlen(msg)) < 0) {
        const char *err = "[ERR] send error\n";
        /* printf isn't signal-safe
         * as this is just error print we do not retry in case of err */
        (void)write(STDOUT_FILENO, err, strlen(err));
        exit(EXIT_FAILURE);
    }
}

static void handler(int sig) {
    struct timespec ts;

    if (clock_gettime(clockid, &ts)) {
        send_msg("clock_gettimeERR: ping\n");
    } else {
        char msg[256];

        snprintf(msg, sizeof(msg) - 1, "%" PRIu64 ".%09lu ping\n",
             (uint64_t)ts.tv_sec, ts.tv_nsec);
        send_msg(msg);
    }
}

#define LLDIGITS10_MAX ((size_t)19)
static long long safe_atoll(const char *str) {
    char *endptr = NULL;
    long long res = strtoll(str, &endptr, 10);

    if(endptr - str != strnlen(str, LLDIGITS10_MAX)) {
        fprintf(stderr, "[ERR] `%s` isn't an integer\n", str);
        exit(EXIT_FAILURE);
    }

    return res;
}

static void print_help(char* argv[]) {
  fprintf(stderr, "Usage: %s <time ms> <repetitions> <clock> [out file]\n",
          argv[0]);
  fprintf(stderr, "supported clocks: \n");
  for(size_t i = 0; i < clock_n; ++i) {
    fprintf(stderr, "- %s\n", clocks[i].name);
  }
}

int main(int argc, char* argv[]) {
    if (argc != 5 && argc != 4) {
        print_help(argv);
        exit(EXIT_FAILURE);
    }

    long long msecs = safe_atoll(argv[1]);
    long long iterations = safe_atoll(argv[2]);

    // sets the global clockid
    for(size_t i = 0; i <= clock_n; ++i) {
        /* found no matching clock */
        if(i == clock_n) {
            print_help(argv);
            fprintf(stderr, "supplied clock: '%s'\n", argv[3]);
            exit(EXIT_FAILURE);
        }

        if(strcmp(argv[3], clocks[i].name) == 0) {
            clockid = clocks[i].id;
            break;
        }
    }

    // change the output descriptor if a 4th argument is supplied
    if (argc == 5) {
        out_desc = open(argv[4], O_WRONLY);
        if(out_desc < 0) {
            fprintf(stderr, "Couldn't open file `%s`, errno: %d\n",
                    argv[4], errno);
            exit(EXIT_FAILURE);
        }
    }

    // Establish handler for timer signal
    struct sigaction sa;
    memset(&sa, 0, sizeof(sa));
    sa.sa_handler = handler;
    sigemptyset(&sa.sa_mask); /* just this sig will be in mask, no SA_NODEFER */
    if (sigaction(SIG, &sa, NULL)) {
        fprintf(stderr, "[ERR] Couldn't set signal disposition, errno: %d\n",
                errno);
        exit(EXIT_FAILURE);
    }

    // Create the timer
    struct sigevent sev;
    timer_t timerid;
    memset(&sev, 0, sizeof(sev));
    sev.sigev_notify = SIGEV_SIGNAL;
    sev.sigev_signo = SIG;
    sev.sigev_value.sival_ptr = &timerid;
    if (timer_create(clockid, &sev, &timerid) == -1) {
        fprintf(stderr, "[ERR] Couldn't create the timer: %d\n", errno);
        exit(EXIT_FAILURE);
    }

    // Start the timer
    struct itimerspec its;
    memset(&its, 0, sizeof(its));
    its.it_value.tv_sec = 0;
    its.it_value.tv_nsec = 10000000; // Timer will not start when 0
    its.it_interval.tv_sec = msecs / 1000;
    its.it_interval.tv_nsec = (msecs % 1000) * 1000000;

    if (timer_settime(timerid, 0, &its, NULL) == -1) {
        fprintf(stderr, "[ERR] Couldn't set timer time: %d\n", errno);
        exit(EXIT_FAILURE);
    }

    for(int i = 0; i < iterations + 1; ++i) {
        if(pause() < 0 && errno == EINTR && out_desc)
            /* in case out_desc is not stdout */
            printf("[INFO] Sent msg\n");
    }

    exit(EXIT_SUCCESS);
}
