# Copyright 2019 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# This app copies an incrementing counter to the clipboard every second.

import gi
gi.require_version('Gtk', '3.0')
from gi.repository import Gtk, Gdk, GObject
import sys

counter = 0

def check():
  global counter
  Gtk.Clipboard.get(Gdk.SELECTION_CLIPBOARD).set_text(str(counter), -1)
  counter += 1
  return True

window = Gtk.Window(title="secure_copy")
window.present()
window.connect('delete-event', Gtk.main_quit)
GObject.timeout_add(1000, check)
Gtk.main()
