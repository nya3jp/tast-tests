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

    tast run $HOST video.Capability*

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

    tast run $HOST video.DecodeAccel{H264,VP8,VP9}*

There are variants of these tests present that have 'VD' in their names. These
tests operate on the new video decoder implementations, which are set to replace
the current ones. To run all VD video decoder tests run:

    tast run $HOST video.DecodeAccelVD*

## Video decoder performance tests

These tests measure video decode performance by running the
[video_decode_accelerator_perf_tests]. These tests are implemented directly on
top of the video decoder implementations and collect various metrics such as
FPS, CPU usage and decode latency. For more information about these tests check
the [video decoder performance tests usage documentation].

Performance tests are available for various codecs using 1080p and 2160p videos,
both in 30 and 60fps variants. To run all performance tests use:

    tast run $HOST video.DecodeAccelPerf*

There are variants of these tests present that have 'VD' in their names. These
tests operate on the new video decoder implementations, which are set to replace
the current ones. To run all VD video decoder performance tests run:

    tast run $HOST video.DecodeAccelVDPerf*

## Video encoder tests

These tests run the [video_encode_accelerator_unittest] to test encoding raw
video frames. They are implemented directly on top of the video encoder
implementations. Tests are available that test encoding H.264, VP8 and VP9
videos using various resolutions.

To run all video encode tests use:

    tast run $HOST video.EncodeAccel{H264,VP8,VP9}*

## Video encoder performance tests

These tests measure video encode performance by running the
[video_encode_accelerator_unittest]. They are implemented directly on top of the
video encoder implementations. Various metrics are collected such as CPU usage.
Tests are available for various codecs and resolutions. To run all tests use:

    tast run $HOST video.EncodeAccelPerf*

## Video play tests

The video play tests verify whether video playback works by playing a video in
the Chrome browser. These tests exercise the full Chrome stack, as opposed to
the video decoder tests that only verify the video decoder implementations. Two
variants of these tests are present.

The _video.Play*_ tests check whether video playback works by any means
possible, fallback on a software video decoder is allowed if hardware video
decoding fails. Tests are available using H.264, VP8 and VP9 videos. To run
these tests use:

    tast run $HOST video.Play{H264,VP8,VP9}*

The _video.PlayDecodeAccelUsed*_ tests are similar to the normal video play
tests. However these tests will only pass if hardware video decoding was
successful. Fallback on a software video decoder is not allowed. Tests are
available for H.264, VP8 and VP9, both for normal videos and videos using MSE.
To run these tests use:

    tast run $HOST video.PlayDecodeAccelUsed{H264,VP8,VP9}* video.PlayDecodeAccelUsedMSE*

Additionally there are variants of these tests with 'VD' in their names present.
These test the new video decoder implementations, which are set to replace the
current ones. To run all VD video play tests run:

    tast run $HOST video.PlayVD* video.PlayDecodeAccelUsedVD*

## Video playback performance tests

The video playback performance tests measure video decoder performance by
playing a video in the Chrome browser. These tests exercise the full Chrome
stack, as opposed to the video decoder performance tests that only measure the
performance of the actual video decoder implementations. Various metrics are
collected such as CPU usage and the number of dropped frames.

Tests are available for various codecs and resolutions, both in 30 and 60fps
variants. To run all tests use:

    tast run $HOST video.PlaybackPerf{AV1,H264,VP8,VP9}*

Additionally there are variants of these tests with 'VD' in their names present.
These test the new video decoder implementations, which are set to replace the
current ones. To run all VD video playback performance tests run:

    tast run $HOST video.PlaybackVDPerf*

## Video decoder sanity checks

These tests use the [video_decode_accelerator_tests] to decode a video stream
with unsupported features. This is done by playing VP9 profile1-3 videos while
the decoder is incorrectly configured for profile0. The tests verify whether a
decoder is able to handle unexpected errors gracefully. To run all sanity checks
use:

    tast run $HOST video.DecodeAccelSanity*

## Video seek tests

These tests check whether seeking in a video works as expected. This is done by
playing a video in the Chrome browser while rapidly jumping between different
points in the video stream. Tests are available for H.264, VP8 and VP9 videos.
In addition there are variants of these tests present that verify seeking in
resolution-changing videos. To run all video seek tests run:

    tast run $HOST video.Seek*

[tast video folder]: https://chromium.googlesource.com/chromiumos/platform/tast-tests/+/refs/heads/master/src/chromiumos/tast/local/bundles/cros/video/
[video_decode_accelerator_tests]: https://cs.chromium.org/chromium/src/media/gpu/video_decode_accelerator_tests.cc
[video decoder tests usage documentation]: https://chromium.googlesource.com/chromium/src/+/master/docs/media/gpu/video_decoder_test_usage.md
[video_decode_accelerator_perf_tests]: https://cs.chromium.org/chromium/src/media/gpu/video_decode_accelerator_perf_tests.cc
[video decoder performance tests usage documentation]: https://chromium.googlesource.com/chromium/src/+/master/docs/media/gpu/video_decoder_perf_test_usage.md
[video_encode_accelerator_unittest]: https://cs.chromium.org/chromium/src/media/gpu/video_encode_accelerator_unittest.cc

