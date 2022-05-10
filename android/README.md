# Android projects used by Tast tests

This directory contains Android projects that are used by Tast tests. They are
built at the same time the ChromeOS test image is built, and not updated by
tast run command automatically.

## Using an APK under the directory in Tast

Take ArcGamepadTest.apk for example.

```go
p := s.PreValue().(arc.PreData)
a := p.ARC

if err := a.Install(ctx, arc.APKPath("ArcGamepadTest.apk")); err != nil {
  s.Fatal("Failed to install the APK: ", err)
}
```

## Updating APKs on the device

```bash
# one time setup
setup_board --board=${BOARD}
./build_packages --board=${BOARD}

cros_workon --board=${BOARD} start tast-local-apks-cros
emerge-${BOARD} tast-local-apks-cros
cros deploy --root=/usr/local ${DUT_IP} tast-local-apks-cros
```

Then you can run normal tast run command.

## Finding the built APKs
In the chroot they can be found under
`/build/$BOARD/usr/libexec/tast/apks/local/cros`

On the DUT they can be found under
`/usr/local/libexec/tast/apks/local/cros/`

