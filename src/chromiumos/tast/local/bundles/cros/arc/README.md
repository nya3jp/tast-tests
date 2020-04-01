# Video Test Overview

The Tast video tests can be used to validate various ARC video decoder and
encoder implementations. A wide range of test scenarios are available to
validate both correctness and performance.

[TOC]


## ARC video encoder tests

These tests validate Android video encoding functionality by running the
[c2_e2e_test]. This test is implemented on top of the Android
[MediaCodec] interface and encodes a raw video stream to verify encoding
functionality.

Currently a test is only available for the H.264 codec. To run the test use:

    tast run $HOST arc.VideoEncodeAccel.*

## ARC video encoder performance tests

These tests measure Android video encoder performance by running the above
[c2_e2e_test]. This test measures the encoder's FPS, bitrate and
latency.

Currently a performance test is only available for the H.264 codec with a 1080p
video stream. To run the test use:

    tast run $HOST arc.VideoEncodeAccelPerf.*
