# Copyright 2019 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# This applications brings up a maximized window and fills it with yellow.

import gi
gi.require_version('Gtk', '3.0')
from gi.repository import Gtk, Gdk

window = Gtk.Window(title="gtk3_demo")
window.modify_bg(Gtk.StateType.NORMAL, Gdk.color_parse("#FFFF00"))

# TODO(crbug.com/994009): Prefer to use maximize, which doesn't work currently.
# This workaround will break if tast tests start running on multi-monitor.
screen = Gdk.Screen.get_default()
window.resize(screen.get_width(), screen.get_height())

window.present()
window.connect('delete-event', Gtk.main_quit)
Gtk.main()
