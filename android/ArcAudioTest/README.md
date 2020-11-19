// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

# ARCAudioTest
ARCAudioTest is an app used in Tast Arc audio tests.

# How to modify ARCAudioTest with Android Studio
1. Launch Android Studio, and click `File > New > Import Project`
2. Locate `chromiumos/src/platform/tast-tests/android/`, click the `build.gradle` to select it, and then click `OK` to import your project.
3. Click `Build > Make Project`.

# How to build ArcAudioTest.apk
## For development
1. Connect your DUT with ADB (ex: `$ adb connect ${DUT_IP}:22` )
2. Select `Run ARCAudioTest` from the `Run` menu.
3. `ARCAudioTest` should build and then appear on your DUT.

# How to test ArcAudioTest.apk with tast

1. Rebuild APK
```
cros_workon --board=${BOARD} start tast-local-apks-cros
emerge-${BOARD} tast-local-apks-cros
cros deploy --root=/usr/local ${DUT_IP} tast-local-apks-cros
```

2. Run your tast testcase in chroot by:
```
$ tast -verbose run ${DUT_IP} arc.AudioValidity.playback
```

# Reference Doc
[Android projects used by Tast tests]


[Android projects used by Tast tests]: https://chromium.googlesource.com/chromiumos/platform/tast-tests/+/HEAD/android/README.md
