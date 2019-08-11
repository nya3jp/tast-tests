# Copyright 2019 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# This applications brings up a maximized window and fills it with magenta.

import sys
from PyQt5.QtWidgets import QApplication, QWidget
from PyQt5.QtGui import QColor

app = QApplication(sys.argv)
w = QWidget()

# TODO(crbug.com/994009): Prefer to use maximize, which doesn't work currently.
# This workaround will break if tast tests start running on multi-monitor.
s = app.primaryScreen().size()
w.setGeometry(0, 0, s.width(), s.height())

p = w.palette()
p.setColor(w.backgroundRole(), QColor(255, 0, 255))
w.setPalette(p)

w.show()
sys.exit(app.exec_())
