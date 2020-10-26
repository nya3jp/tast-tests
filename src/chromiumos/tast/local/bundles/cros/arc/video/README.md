# Video Test Overview

The Tast video tests can be used to validate various ARC video decoder and
encoder implementations. A wide range of test scenarios are available to
validate both correctness and performance.

To get a list of all available ARC video tests run:

    tast list $HOST | grep arc.Video

[TOC]

## ARC video decoder tests

These tests validate Android video decoding functionality by running the
[c2_e2e_test]. This test is implemented on top of the Android
[MediaCodec] interface. The test decodes a video from start to finish and
validates decoded frames by comparing their checksums against expected values.

Tests are available for the H.264, VP8 and VP9 codecs. To run all tests use:

    tast run $HOST arc.VideoDecodeAccel.*

## ARC video decoder performance tests

These tests measure Android video decoder performance by running the above
[c2_e2e_test]. Currently the performance tests measures the
decoder's maximum FPS by decoding a video as fast as possible, and it measures
cpu, power usage, and dropped frames while decoding and rendering a video at
the appropriate FPS.

Performance tests are available for the H.264, VP8 and VP9 codecs, using 1080p
and 2160p videos, both in 30 and 60fps variants. To run all performance tests
use:

    tast run $HOST arc.VideoDecodeAccelPerf.*

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
