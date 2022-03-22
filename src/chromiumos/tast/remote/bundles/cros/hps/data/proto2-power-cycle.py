#!/usr/bin/env python3

# Copyright 2022 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# Turns the HPS off then back on again. This is useful after flashing the HPS
# MCU for the first time, or after running a factory test in order to get back
# to the system bootloader (provided the MCU flash is empty).

from pyftdi.gpio import GpioAsyncController
import time
import argparse

parser = argparse.ArgumentParser(description='HPS dev board controller')
parser.add_argument(
    '--write-protect', action='store_true',
    help='Enable write protect.')
args = parser.parse_args()

gpio = GpioAsyncController()
power_pin = 1 << 6
write_protect_pin = 1 << 4
direction = power_pin
# Both power enable and firmware write-protect have pull-up resistors, so we set
# them low by making the pins outputs and pulling them low, but we set them high
# by making them inputs and letting the pull-ups do their work.
if not args.write_protect:
    direction |= write_protect_pin
gpio.configure('ftdi://ftdi:ft4232/1', direction=direction, initial=0)
time.sleep(0.1)
# Set GPIO pin 6 back to being an input. It'll get pulled high by the pull-up
# resistor (power on).
gpio.set_direction(power_pin, 0)
