#!/usr/bin/env python3
# -*- coding: utf-8 -*-
# Copyright 2019 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Python wrapper for dump_syms.

path/to/dump_syms.py input output
"""

from __future__ import print_function

import subprocess
import sys

print("1=" + sys.argv[1])
print("2=" + sys.argv[2])
print("trying to LS the file:" + sys.argv[1])
subprocess.check_call(["echo", "ls", sys.argv[1]])
subprocess.check_call(["ls", sys.argv[1]])

with open(sys.argv[2], "w") as outfile:
    subprocess.check_call(["dump_syms", sys.argv[1]], stdout=outfile)
