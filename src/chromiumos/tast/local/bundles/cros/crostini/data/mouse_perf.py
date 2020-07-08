# Copyright 2020 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# This applications tracks the movement of the mouse and prints its location.

import gi
gi.require_version('Gtk', '3.0')
from gi.repository import Gtk, Gdk

def on_pointer_motion(widget, event):
  print(event.x)
  print(event.y)
  print(event.time)

window = Gtk.Window(title="mouse_perf")
window.modify_bg(Gtk.StateType.NORMAL, Gdk.color_parse("#7F00FF"))
window.maximize()
window.present()
window.connect('delete-event', Gtk.main_quit)
window.connect('motion_notify_event', on_pointer_motion)
Gtk.main()
