#!/bin/bash
# Copyright 2020 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# Replay the input events on DUT by evemu-play from pre-generated kernel events.

echo "Input Device: $1"
echo "Replay Sequence: $2"

evemu-play "$1" < "$2"