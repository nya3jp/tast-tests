# FAFT for bringup (go/faft-bringup)

[TOC]

## Setup

> This will be much simpler once b/197287681 is fixed.

You need a servo_v4 or servo_v4.1, probably a servo micro, or a C2D2 micro, the
board you are testing, and a spare chromebook with an ethernet adapter.

The spare chromebook is just to workaround tast insisting on SSHing to the DUT
before running any tests. It needs to be the same arch (x86/arm) and preferably
the same form factor (i.e. clamshell).

## Running a single test
```
(inside chroot)
SERVO=localhost:9999:nossh
SPARE_CHROMEBOOK=192.168.1.78
BOARD=grunt
MODEL=treeya
tast run --var servo=${SERVO?} --var noSSH=true --var board=${BOARD?} --var model=${MODEL?} ${SPARE_CHROMEBOOK?} firmware.ECPowerG3
```

## Running all tests

```
(inside chroot)
SERVO=localhost:9999:nossh
SPARE_CHROMEBOOK=192.168.1.78
BOARD=grunt
MODEL=treeya
tast run --var servo=${SERVO?} --var noSSH=true --var board=${BOARD?} --var model=${MODEL?} ${SPARE_CHROMEBOOK?} '("group:firmware" && firmware_bringup)'
```

if you want to exclude slow tests

```
tast run --var servo=${SERVO?} --var noSSH=true --var board=${BOARD?} --var model=${MODEL?} ${SPARE_CHROMEBOOK?} '("group:firmware" && firmware_bringup && !firmware_slow)'
```

## Adapting a firmware test for bringup

Follow the instructions for creating a [FAFT test in Tast](codelab/README.md).
For the bringup case, you must not make any SSH calls to the DUT, and that means
that you can't use any [Tast gRPC
services](https://chromium.googlesource.com/chromiumos/platform/tast/+/HEAD/docs/writing_tests.md#calling-grpc-services)
either.

### Getting board specific configs

Set the board and model explicitly from command line vars:

```
func init() {
	testing.AddTest(&testing.Test{
                Func: MyTest,
		Vars: []string{"board", "model"},
                // ...
	})
}

func MyTest(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper

	board, _ := s.Var("board")
	model, _ := s.Var("model")
	h.OverridePlatform(ctx, board, model)

	if err := h.RequireConfig(ctx); err != nil {
		s.Fatal("Failed to get config: ", err)
	}

        // At this point h.Config, h.Board, and h.Model are usable.
}
```
