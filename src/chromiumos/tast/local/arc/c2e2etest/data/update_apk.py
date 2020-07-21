#!/usr/bin/env python3
# Copyright 2020 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Update C2E2ETest.apk .external file."""

import logging
import subprocess
import tempfile
from typing import Iterable
import os

GS_ROOT = 'gs://chromeos-arc-images/builds'
ANDROID_BRANCH = 'rvc-arc-dev'
DEVICE_NAME = 'bertha'
ANDROID_BUILD_TYPE = 'user'
X86_64 = 'x86_64'
ARM = 'arm'
APK_NAME = 'C2E2ETest.apk'


def get_root_dir(is_x86) -> str:
    """asdf. """

    arch = X86_64 if is_x86 else ARM
    return '%s/git_%s-linux-%s_%s-%s/' % (
        GS_ROOT, ANDROID_BRANCH, DEVICE_NAME, arch, ANDROID_BUILD_TYPE)


def list_dir(dir_path) -> Iterable[str]:
    """asdf. """

    def trip_dir_path(item_path) -> str:
        """asdf. """
        if not item_path.startswith(dir_path):
            logging.error('ls get weird result: inside %s we got %s',
                          dir_path, item_path)
            return ''
        item_name = item_path[len(dir_path):]
        if item_name[-1] == '/':
            item_name = item_name[:-1]
        return item_name

    output = subprocess.check_output(['gsutil', 'ls', dir_path],
                                     encoding='UTF-8')
    return map(trip_dir_path, output.splitlines())


def get_file_info(file_path):
    """asdf. """

    file_name = os.path.basename(file_path)
    with tempfile.TemporaryDirectory() as temp_dir:
        subprocess.check_call(['gsutil', 'cp', file_path, temp_dir])
        temp_file = os.path.join(temp_dir, file_name)
        file_size = os.path.getsize(temp_file)
        print(file_size)
        sha256sum = subprocess.check_output(['sha256sum', temp_file],
                                            encoding='UTF-8')
        print(sha256sum)


def main():
    """asdf. """

    logging.basicConfig(level=logging.INFO)

    logging.info('x86 root: %s, arm root: %s', get_root_dir(is_x86=True),
                 get_root_dir(is_x86=False),)

    max_version = max(map(int, list_dir(get_root_dir(is_x86=True))))
    print(max_version)



if __name__ == '__main__':
    main()
