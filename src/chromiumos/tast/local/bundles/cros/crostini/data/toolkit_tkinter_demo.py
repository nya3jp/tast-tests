# Copyright 2019 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# This applications brings up a maximized window and fills it with cyan.

from tkinter import Tk

root = Tk(className="tkinter_demo")
root['bg'] = "#00FFFF"

# TODO(crbug.com/994009): Prefer to use maximize, which doesn't work currently.
# This workaround will break if tast tests start running on multi-monitor.
w, h = root.winfo_screenwidth(), root.winfo_screenheight()
root.geometry("%dx%d+0+0" % (w, h))

root.mainloop()
