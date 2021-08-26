# How to bisect lacros tests manually

This document describes how to bisect lacros tests manually. Because the lacros
binary in Tast is not uprevved at the same cadence as the Ash binary, it is
sometimes necessary to bisect lacros revisions manually.

## Conventions

| Label       | Paths, files, and commands           |
|-------------|--------------------------------------|
|  (cros)     | inside the Chromium OS chroot        |
|  (chrm)     | inside the Chromium source directory |

## Bisect setup

First start off the bisect:

```sh
(chrm) git checkout <some bad revision>
(chrm) git bisect start
(chrm) git bisect bad
(chrm) git checkout <some good revision>
(chrm) git bisect good
```

Then set up some variables in both the cros chroot and Chromium source directory
to make it easier to copy-paste commands from this document:

```sh
export LACROS_DIR=out/Lacros
export DUT=<IP and optional port of your device-under-test>
export TEST=lacros.Basic
```

## Bisect loop

First, build lacros Chrome at the current bisect version, and deploy it:

```sh
(chrm) gclient sync -D -r HEAD -j 256
(chrm) autoninja -C $LACROS_DIR chrome
(chrm) ./third_party/chromite/bin/deploy_chrome --build-dir=$LACROS_DIR -d $DUT --lacros --nostrip
```

Optionally, you may have to deploy the Chromium OS Chromium version as well:

```sh
(chrm) export BOARD=<your board>
(chrm) autoninja -C out_$BOARD chrome
(chrm) ./third_party/chromite/bin/deploy_chrome --build-dir=out_${BOARD}/Release --device=$DUT --nostrip --board=${BOARD}
```

Then run the given Tast test. N.B. that the Tast test must use the
lacros fixture or a variant of it for setting
`lacrosDeployedBinary` to work.

```sh
(cros) tast run -var lacrosDeployedBinary=/usr/local/lacros-chrome $DUT $TEST
```

Based on the result, continue the bisect:

```
(chrm) git bisect <good or bad>
```
