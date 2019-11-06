# Video Test Overview

The Tast video tests can be used to validate various video decoder and encoder
implementations. A wide range of test scenarios are available to validate both
correctness and performance. Some tests operate directly on top of the decoder
and encoder implementations, while other tests operate on the entire Chrome
stack.

To get a list of all available video tests run:

    tast list $HOST | grep video.

All video tests can be found in the [tast video folder].

[TOC]

## Capability check test

This test checks whether a device reports the correct set of capabilities (e.g.
VP9 support). It can be run by executing:

    tast run $HOST video.Capability

## Video decoder tests

These tests validate video decoding functionality by running the
[video_decode_accelerator_tests]. They are implemented directly on top of the
video decoder implementations. Various behaviors are tested such as flushing and
resetting the decoder. Decoded frames are validated by comparing their checksums
against expected values. For more information about these tests check the
[video decoder tests usage documentation].

Tests are available for various codecs such as H.264, VP8 and VP9. In addition
there are tests using videos that change resolution during plaback. To run all
tests use:

    tast run $HOST video.DecodeAccel.*

There are variants of these tests present that have 'VD' in their names. These
tests operate on the new video decoder implementations, which are set to replace
the current ones. To run all VD video decoder tests run:

    tast run $HOST video.DecodeAccelVD.*

## Video decoder performance tests

These tests measure video decode performance by running the
[video_decode_accelerator_perf_tests]. These tests are implemented directly on
top of the video decoder implementations and collect various metrics such as
FPS, CPU usage, power consumption (Intel devices only) and decode latency. For
more information about these tests check the
[video decoder performance tests usage documentation].

Performance tests are available for various codecs using 1080p and 2160p videos,
both in 30 and 60fps variants. To run all performance tests use:

    tast run $HOST video.DecodeAccelPerf.*

There are variants of these tests present that have 'VD' in their names. These
tests operate on the new video decoder implementations, which are set to replace
the current ones. To run all VD video decoder performance tests run:

    tast run $HOST video.DecodeAccelVDPerf.*

## Video encoder tests

These tests run the [video_encode_accelerator_unittest] to test encoding raw
video frames. They are implemented directly on top of the video encoder
implementations. Tests are available that test encoding H.264, VP8 and VP9
videos using various resolutions.

To run all video encode tests use:

    tast run $HOST video.EncodeAccel.*

## Video encoder performance tests

These tests measure video encode performance by running the
[video_encode_accelerator_unittest]. They are implemented directly on top of the
video encoder implementations. Various metrics are collected such as CPU usage.
Tests are available for various codecs and resolutions. To run all tests use:

    tast run $HOST video.EncodeAccelPerf.*

## Video play tests

The video play tests verify whether video playback works by playing a video in
the Chrome browser. These tests exercise the full Chrome stack, as opposed to
the video decoder tests that only verify the video decoder implementations. Two
variants of these tests are present.

The _video.Play.*_ tests check whether video playback works by any means
possible, fallback on a software video decoder is allowed if hardware video
decoding fails. Tests are available using H.264, VP8 and VP9 videos. To run
these tests use:

    tast run $HOST video.Play.*

The _video.PlayDecodeAccelUsed.*_ tests are similar to the normal video play
tests. However these tests will only pass if hardware video decoding was
successful. Fallback on a software video decoder is not allowed. Tests are
available for H.264, VP8 and VP9, both for normal videos and videos using MSE.
To run these tests use:

    tast run $HOST video.PlayDecodeAccelUsed.*

Additionally there are variants of these tests with 'VD' in their names present.
These test the new video decoder implementations, which are set to replace the
current ones. To run all VD video play tests run:

    tast run $HOST video.PlayVD.* video.PlayDecodeAccelUsedVD.*

## Video playback performance tests

The video playback performance tests measure video decoder performance by
playing a video in the Chrome browser. These tests exercise the full Chrome
stack, as opposed to the video decoder performance tests that only measure the
performance of the actual video decoder implementations. Both software and
hardware video decoder performance is measured. If hardware decoding is not
supported for the video stream only software performance will be reported.
Various metrics are collected such as CPU usage and the number of dropped
frames.

Tests are available for various codecs and resolutions, both in 30 and 60fps
variants. To run all tests use:

    tast run $HOST video.PlaybackPerf.*

Additionally there are variants of these tests with 'VD' in their names present.
These test the new video decoder implementations, which are set to replace the
current ones. To run all VD video playback performance tests run:

    tast run $HOST video.PlaybackVDPerf.*

## Video decoder sanity checks

These tests use the [video_decode_accelerator_tests] to decode a video stream
with unsupported features. This is done by playing VP9 profile1-3 videos while
the decoder is incorrectly configured for profile0. The tests verify whether a
decoder is able to handle unexpected errors gracefully. To run all sanity checks
use:

    tast run $HOST video.DecodeAccelSanity.*

## Video seek tests

These tests check whether seeking in a video works as expected. This is done by
playing a video in the Chrome browser while rapidly jumping between different
points in the video stream. Tests are available for H.264, VP8 and VP9 videos.
In addition there are variants of these tests present that verify seeking in
resolution-changing videos. To run all video seek tests run:

    tast run $HOST video.Seek*

## ARC video decoder tests

These tests validate Android video decoding functionality by running the
[arc_video_decoder_e2e_test]. This test is implemented on top of the Android
[MediaCodec] interface. The test decodes a video from start to finish and
validates decoded frames by comparing their checksums against expected values.

Tests are available for the H.264, VP8 and VP9 codecs. To run all tests use:

    tast run $HOST video.ARCDecodeAccel.*

## ARC video decoder performance tests

These tests measure Android video decoder performance by running the above
[arc_video_decoder_e2e_test]. Currently the performance tests measures the
decoder's maximum FPS by decoding a video as fast as possible, and it measures
cpu, power usage, and dropped frames while decoding and rendering a video at
the appropriate FPS.

Performance tests are available for the H.264, VP8 and VP9 codecs. To run all
performance tests use:

    tast run $HOST video.ARCDecodeAccelPerf.*

## ARC video encoder tests

These tests validate Android video encoding functionality by running the
[arc_video_encoder_e2e_test]. This test is implemented on top of the Android
[MediaCodec] interface and encodes a raw video stream to verify encoding
functionality.

Currently a test is only available for the H.264 codec. To run the test use:

    tast run $HOST video.ARCEncodeAccel.*

## ARC video encoder performance tests

These tests measure Android video encoder performance by running the above
[arc_video_encoder_e2e_test]. This test measures the encoder's FPS, bitrate and
latency.

Currently a performance test is only available for the H.264 codec with a 1080p
video stream. To run the test use:

    tast run $HOST video.ARCEncodeAccelPerf.*

[tast video folder]: https://chromium.googlesource.com/chromiumos/platform/tast-tests/+/refs/heads/master/src/chromiumos/tast/local/bundles/cros/video/
[video_decode_accelerator_tests]: https://cs.chromium.org/chromium/src/media/gpu/video_decode_accelerator_tests.cc
[video decoder tests usage documentation]: https://chromium.googlesource.com/chromium/src/+/master/docs/media/gpu/video_decoder_test_usage.md
[video_decode_accelerator_perf_tests]: https://cs.chromium.org/chromium/src/media/gpu/video_decode_accelerator_perf_tests.cc
[video decoder performance tests usage documentation]: https://chromium.googlesource.com/chromium/src/+/master/docs/media/gpu/video_decoder_perf_test_usage.md
[video_encode_accelerator_unittest]: https://cs.chromium.org/chromium/src/media/gpu/video_encode_accelerator_unittest.cc
[arc_video_decoder_e2e_test]: https://googleplex-android.googlesource.com/platform/external/v4l2_codec2/+/refs/heads/pi-arc/tests/c2_e2e_test/
[arc_video_encoder_e2e_test]: https://chromium.googlesource.com/chromiumos/platform2/+/master/arc/codec-test/
[MediaCodec]: https://developer.android.com/reference/android/media/MediaCodec
