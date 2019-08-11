# Copyright 2019 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

import gi
gi.require_version('Gtk', '3.0')
from gi.repository import Gtk, Gdk

# TODO(hollingum): This only works in a 1-display environment. If tast tests
# start running on multi-monitor, we'll need to adjust.
screen = Gdk.Screen.get_default()

window = Gtk.Window(title="gtk3_demo")
window.modify_bg(Gtk.StateType.NORMAL, Gdk.color_parse("#FFFF00"))
window.resize(screen.get_width(), screen.get_height())

window.present()
window.connect('delete-event', Gtk.main_quit)
Gtk.main()
