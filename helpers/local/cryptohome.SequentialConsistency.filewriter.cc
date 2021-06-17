// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

#include <errno.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>
#include <sys/types.h>
#include <unistd.h>

#include <string>

#include <base/files/file.h>
#include <base/files/file_path.h>
#include <base/logging.h>
#include <base/strings/stringprintf.h>
#include <brillo/flag_helper.h>
#include <brillo/syslog_logging.h>

namespace {
// Both of these should match sequential_consistency.go.
const int kNumFiles = 9;
const char kBasePhrase[] = "This is file #%d";

// makeFilePath here should match makeFilePath in sequential_consistency.go.
base::FilePath makeFilePath(const base::FilePath &path, int file_num) {
  return path.Append(
      base::StringPrintf("SequentialConsistencyTest.%d.txt", file_num));
}

bool createFile(const base::FilePath &path, int file_num, bool slowly = false) {
  base::FilePath file_path = makeFilePath(path, file_num);
  base::File f(file_path,
               base::File::FLAG_CREATE_ALWAYS | base::File::FLAG_WRITE);
  if (!f.IsValid()) {
    LOG(ERROR) << "file open(" << file_path.value()
               << "): " << base::File::ErrorToString(f.error_details());
    return false;
  }

  std::string phrase = base::StringPrintf(kBasePhrase, file_num);
  if (slowly) {
    for (int i = 0; i < phrase.length(); i++) {
      char c = phrase[i];
      int result = f.WriteAtCurrentPos(&c, 1);
      if (result != 1) {
        LOG(ERROR) << file_path.value() << " WriteAtCurrentPos() returned "
                   << result << ": "
                   << base::File::ErrorToString(base::File::GetLastFileError());
        return false;
      }
    }
  } else {
    int result = f.WriteAtCurrentPos(phrase.c_str(), phrase.length());
    if (result != phrase.length()) {
      LOG(ERROR) << file_path.value() << " WriteAtCurrentPos() returned "
                 << result << ": "
                 << base::File::ErrorToString(base::File::GetLastFileError());
      return false;
    }
  }
  return true;
}

int createFiles(bool create_dir, const base::FilePath &path) {
  // Make sure the test is ready for us.
  sleep(2);

  if (create_dir) {
    // base::CreateDirectory doesn't support permissions
    if (mkdir(path.value().c_str(), 0755) != 0) {
      PLOG(ERROR) << "mkdir(" << path.value() << "): ";
      return EXIT_FAILURE;
    }
  }

  int file_num = 0;
  // Immediately create first three files.
  for (; file_num < 3; ++file_num) {
    if (!createFile(path, file_num)) {
      return EXIT_FAILURE;
    }
  }

  // Sleep briefly and create the next three.
  sleep(2);
  for (; file_num < 6; ++file_num) {
    if (!createFile(path, file_num)) {
      return EXIT_FAILURE;
    }
  }

  // Write out the remaining three slowly.
  for (; file_num < kNumFiles; ++file_num) {
    if (!createFile(path, file_num, true)) {
      return EXIT_FAILURE;
    }
  }

  return EXIT_SUCCESS;
}
} // namespace

int main(int argc, char **argv) {
  brillo::InitLog(brillo::kLogToSyslog | brillo::kLogToStderrIfTty);

  DEFINE_bool(create_dir, false, "Create directory given by path");
  DEFINE_string(path, "", "Directory to put files in");
  brillo::FlagHelper::Init(argc, argv,
                           "cryptohome.SequentialConsistency tast test helper");
  CHECK_NE(FLAGS_path, "");

  // Fork to detach process from parent.
  pid_t fork_result = fork();
  if (fork_result == -1) {
    perror("fork");
    return EXIT_FAILURE;
  }

  if (fork_result == 0) {
    // Child.
    return createFiles(FLAGS_create_dir, base::FilePath(FLAGS_path));
  } else {
    return EXIT_SUCCESS;
  }
}
