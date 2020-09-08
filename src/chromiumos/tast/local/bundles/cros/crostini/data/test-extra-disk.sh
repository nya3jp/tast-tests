#!/bin/bash -ex
# Copyright 2020 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

cd /mnt/external/0
cur_dir=$(pwd)
if [[ "${cur_dir}" != "/mnt/external/0" ]]; then
    echo "Failed to move to the exteral disk: ${cur_dir}"
    exit 1
fi

echo "Hello" > ./hello.txt
content=$(cat hello.txt)
if [[ "${content}" != "Hello" ]]; then
    echo "Failed to read hello.txt: ${content}"
    exit 1
fi

echo "OK"
exit 0
