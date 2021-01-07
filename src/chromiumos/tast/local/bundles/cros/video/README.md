# Video Test Overview ([tinyurl.com/cros-gfx-video](https://tinyurl.com/cros-gfx-video))

The Tast video tests are used to validate various video related functionality. A
wide range of test scenarios are available to validate both correctness and
performance. Some tests operate directly on top of the decoder and encoder
implementations, while other tests operate on the entire Chrome stack.

To get a list of all available video tests run:

    tast list $HOST | grep video.

All video tests can be found in the [tast video folder].

Some tests use Chrome while others do not; a hypothetical user would not see any
action in a DUT when running the latter.

[TOC]

## Capabilities and Capability Test

Tast can prevent running tests on SoCs without a particular functionality (see
[Test Dependencies], and the video tests make extensive use of this for gating
tests on hardware capabilities e.g. presence of video decoding for a given
codec. These capabilities are specified per-chipset (potentially with per-board
and per-device overlays) in files like e.g. [15-chipset-cml-capabilities.yaml].
These files are ingested in Go via the [`caps` package], so that test cases can
use them as preconditions in their `SoftwareDeps`.

For example: Intel Broadwell has support for hardware accelerated H.264 decoding
but not VP8. Therefore, the Broadwell associated capabilities file
[15-chipset-bdw-capabilities.yaml] has a number of `hw_dec_h264_*` entries and
no `hw_dec_vp8_*` entries. Tast ingests these capabilities file(s) and provides
them for tests with the name correspondence defined in the mentioned [`caps`
package]. Any test (case) with a `caps.HWDecodeH264` listed in its
`SoftwareDeps`, for example [`Play.h264_hw`], will thus run on Broadwell
devices, whereas those with `caps.HWDecodeVP8` listed will not, for example
[`Play.vp8_hw`].

Googlers can refer to go/crosvideocodec for more information about the hardware
supported video features.

The Capability test verifies that the capabilities provided by the YAML file(s)
are indeed detected by the hardware via the command line utility
`avtest_label_detect` that is installed in all test/dev images. This test can be
run by executing:

    tast run $HOST video.Capability

New boards being brought up will need to add capabilities files in the
appropriate locations for the Video Tast tests to run properly.

## Video decoder integration tests (`video.DecodeAccel`)

These tests validate video decoding functionality by running the
[video_decode_accelerator_tests]. They are implemented directly on top of the
video decoder implementations, not using Chrome. Various behaviors are tested
such as flushing and resetting the decoder. Decoded frames are validated by
comparing their checksums against expected values. For more information about
these tests check the [video decoder tests usage documentation].

Tests are available for various codecs such as H.264, VP8 and VP9. In addition
there are tests using videos that change resolution during plaback. To run all
tests use:

    tast run $HOST video.DecodeAccel.*

There are variants of these tests present that have 'VD' in their names. These
tests operate on the new video decoder implementations, which are set to replace
the current ones. To run all VD video decoder tests run:

    tast run $HOST video.DecodeAccelVD.*

### Video decoder compliance tests (`video.DecodeCompliance`)

These tests validate video decoding compliance by running the
[video_decode_accelerator_tests] with various video clips and
--gtest\_filter=VideoDecoderTest.FlushAtEndOfStream.
Unlike the DecodeAccel and DecodeAccelVD tests, DecodeCompliance mostly targets
specific codec features and is primarily concerned with the correctness of the
produced frames.
Currently, we only test AV1. To run the test use:

    tast run $HOST video.DecodeCompliance.av1_test_vectors

Please see [data/test_vectors/README.md] for details about the video clips used
in this test.

### Video decoder performance tests (`video.DecodeAccelPerf`)

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

### Video decoder smoke checks

These tests use the [video_decode_accelerator_tests] to decode a video stream
with unsupported features. This is done by playing VP9 profile1-3 videos while
the decoder is incorrectly configured for profile0. The tests verify whether a
decoder is able to handle unexpected errors gracefully. To run all smoke checks
use:

    tast run $HOST video.DecodeAccelSmoke.*

## Video encoder integration tests (`video.EncodeAccel`)

These tests run the [video_encode_accelerator_unittest] to test encoding raw
video frames. They are implemented directly on top of the video encoder
implementations, not using Chrome. Tests are available that test encoding H.264,
VP8 and VP9 videos using various resolutions.

To run all video encode tests use:

    tast run $HOST video.EncodeAccel.*

### Video encoder performance tests (`video.EncodeAccelPerf`)

These tests measure video encode performance by running the
[video_encode_accelerator_unittest]. They are implemented directly on top of the
video encoder implementations. Various metrics are collected such as CPU usage.
Tests are available for various codecs and resolutions. To run all tests use:

    tast run $HOST video.EncodeAccelPerf.*

## PlatformV4L2 Tests (`video.PlatformV4L2`)

V4L2 is a kernel video acceleration API. It is implemented in the kernel
linuxtv sub-tree, and it's used in Chrome for ARM-based boards such as those
shipped by Rockchip, MediaTek, and Qualcomm, and for the companion accelerator
chip Kepler.

### V4L2 video compliance tests (`video.PlatformV4L2.decoder`)

These tests validate the Video4Linux2 video acceleration APIs, for both
input and output. They attempt to verify that all expected IOCTLs are
indeed supported.

To run these tests use:

    tast run $HOST video.PlatformV4L2.decoder

## Video acceleration (VA) API unit test (`video.PlatformVAAPIUnittest`)

This test runs the `test_va_api` GTest binary from the libva-test
package. It checks the libva API against the driver implementation. See
https://github.com/intel/libva-utils for more details.

To run this test use:

    tast run $HOST video.PlatformVAAPIUnittest

## Play Tests (`video.Play`)

The Play test verifies whether video playback works by playing a video in the
Chrome browser. This test exercises the full Chrome stack, as opposed to others
that might only exercise the video decoder implementations. This test has
multiple variants, specified by the test case name parts. Every test case has a
codec part, e.g. `h264` or `av1` followed by none, one or several identifiers
(e.g. `hw` or `mse`).

- Cases without any identifier besides the codec name verify whether video
playback works by any means possible: fallback on a software video decoder is
allowed if hardware video decoding fails. These are basically the vanilla
variants/cases.

- Tests with a `guest` identifier use a guest ChromeOS login, versus those
without the identifier that use the normal test user login. Guest logins have a
different Chrome flag management.

- Tests with a `hw` identifier verify that hardware video decoding was used when
expected, as per the SoC capabilities (see
[Capabilities and Capability Test](#capabilities-and-capability-test)
Section), with fallback on a software
video decoder being treated as an error. Conversely, the tests with a `sw`
identifier force and verify the usage of a software video decoder, such as
libvpx or ffmpeg.

- Tests with an `mse` identifier use MSE (Media Source Extensions protocol) for
video playback, as opposed to those without the identifier that use a simple
file URL.

- Tests with the `alt` identifier employ an alternative hardware video decoding
implementation (see [tinyurl.com/chromeos-video-decoders](https://tinyurl.com/chromeos-video-decoders)).

To run these tests use:

    tast run $HOST video.Play.*

### Play Performance Tests (`video.PlaybackPerf`)

The video playback performance tests measure video decoder performance by
playing a video in the Chrome browser. These tests exercise the full Chrome
stack, as opposed to others that might only measure the performance of the
actual video decoder implementations. Both software and hardware video decoder
performance is measured. Various metrics are collected such as GPU usage, power
consumption and the number of dropped frames.

Tests are available for various codecs and resolutions, both in 30 and 60fps
variants. To run all tests use:

    tast run $HOST video.PlaybackPerf.*

## Seek Tests (`video.Seek`)

These tests verify seeking in videos: Seeks are issued while playing a video in
Chrome, waiting for the `onseeked` event to be received. These tests come in
variants with the same taxonomy as described in the
[Play Tests](#play-tests) Section, and in addition:

- Tests with a `stress` suffix issue a much larger amount of seeks, and have a
much larger timeout.

- Tests with a `switch` suffix utilize a resolution-changing video as input, to
introduce further stress in the video decoder implementations.

To run all video seek tests run:

    tast run $HOST video.Seek.*

### Resolution Ladder Sequence Creation

The `smpte_bars_resolution_ladder.*` videos are generated using a combination of
gstreamer and ffmpeg scripts, concretely for AV1, VP8, VP9 and H.264 (AVC1),
respectively:

    gst-launch-1.0 -e videotestsrc num-buffers=60 pattern=smpte100 ! timeoverlay ! video/x-raw,format=I420,width=320,height=240   ! av1enc ! video/x-av1,profile=main ! webmmux ! filesink location=smpte00.webm;
    gst-launch-1.0 -e videotestsrc num-buffers=60 pattern=smpte100 ! timeoverlay ! video/x-raw,format=I420,width=854,height=480   ! av1enc ! video/x-av1,profile=main ! webmmux ! filesink location=smpte01.webm;
    gst-launch-1.0 -e videotestsrc num-buffers=60 pattern=smpte100 ! timeoverlay ! video/x-raw,format=I420,width=1280,height=800  ! av1enc ! video/x-av1,profile=main ! webmmux ! filesink location=smpte02.webm;
    gst-launch-1.0 -e videotestsrc num-buffers=60 pattern=smpte100 ! timeoverlay ! video/x-raw,format=I420,width=1904,height=1008 ! av1enc ! video/x-av1,profile=main ! webmmux ! filesink location=smpte03.webm;

    gst-launch-1.0 -e videotestsrc num-buffers=60 pattern=smpte100 ! timeoverlay ! video/x-raw,width=320,height=240   ! vp8enc ! "video/x-vp8" ! webmmux ! filesink location=smpte00.vp8.webm;
    gst-launch-1.0 -e videotestsrc num-buffers=60 pattern=smpte100 ! timeoverlay ! video/x-raw,width=854,height=480   ! vp8enc ! "video/x-vp8" ! webmmux ! filesink location=smpte01.vp8.webm;
    gst-launch-1.0 -e videotestsrc num-buffers=60 pattern=smpte100 ! timeoverlay ! video/x-raw,width=1280,height=800  ! vp8enc ! "video/x-vp8" ! webmmux ! filesink location=smpte02.vp8.webm;
    gst-launch-1.0 -e videotestsrc num-buffers=60 pattern=smpte100 ! timeoverlay ! video/x-raw,width=1904,height=1008 ! vp8enc ! "video/x-vp8" ! webmmux ! filesink location=smpte03.vp8.webm;

    gst-launch-1.0 -e videotestsrc num-buffers=60 pattern=smpte100 ! timeoverlay ! video/x-raw,width=320,height=240   ! vp9enc ! "video/x-vp9" ! webmmux ! filesink location=smpte00.vp9.webm;
    gst-launch-1.0 -e videotestsrc num-buffers=60 pattern=smpte100 ! timeoverlay ! video/x-raw,width=854,height=480   ! vp9enc ! "video/x-vp9" ! webmmux ! filesink location=smpte01.vp9.webm;
    gst-launch-1.0 -e videotestsrc num-buffers=60 pattern=smpte100 ! timeoverlay ! video/x-raw,width=1280,height=800  ! vp9enc ! "video/x-vp9" ! webmmux ! filesink location=smpte02.vp9.webm;
    gst-launch-1.0 -e videotestsrc num-buffers=60 pattern=smpte100 ! timeoverlay ! video/x-raw,width=1904,height=1008 ! vp9enc ! "video/x-vp9" ! webmmux ! filesink location=smpte03.vp9.webm;

    gst-launch-1.0 -e videotestsrc num-buffers=60 pattern=smpte100 ! timeoverlay ! video/x-raw,width=320,height=240   ! x264enc ! video/x-h264,profile=baseline ! mp4mux ! filesink location=smpte00.mp4;
    gst-launch-1.0 -e videotestsrc num-buffers=60 pattern=smpte100 ! timeoverlay ! video/x-raw,width=854,height=480   ! x264enc ! video/x-h264,profile=main     ! mp4mux ! filesink location=smpte01.mp4;
    gst-launch-1.0 -e videotestsrc num-buffers=60 pattern=smpte100 ! timeoverlay ! video/x-raw,width=1280,height=800  ! x264enc ! video/x-h264,profile=baseline ! mp4mux ! filesink location=smpte02.mp4;
    gst-launch-1.0 -e videotestsrc num-buffers=60 pattern=smpte100 ! timeoverlay ! video/x-raw,width=1904,height=1008 ! x264enc ! video/x-h264,profile=high     ! mp4mux ! filesink location=smpte03.mp4;

The resolutions were chosen to be of different aspect ratios: `4:3`, `16:9`,
`16:10` and `17:9`, respectively.

Once the subsequences are generated, they are concatenated by adding them in a
text file which contents would then be e.g.

    file 'smpte00.mp4'
    file 'smpte01.mp4'
    file 'smpte02.mp4'
    file 'smpte03.mp4'
    file 'smpte02.mp4'
    file 'smpte01.mp4'
    file 'smpte00.mp4'

which is then used for `ffmpeg`, for example for the MPEG-4 output:

    ffmpeg -f concat -i files.mp4.txt -bsf:v "h264_metadata=level=auto" -c copy smpte.mp4

The line for AV1, VP8 and VP9 is similar, without the `-bsf:v`.

## Canvas Tests (`video.DrawOnCanvas`)

This group of tests verifies that a video (presumably decoded using hardware
acceleration) can be drawn onto a 2D canvas. These tests exercise the full
Chrome stack. This path is important because it's used to display video
thumbnails is the Camera Capture App (CCA).

The tests are mostly driven by [video-on-canvas.html] which loads a video,
starts playing it, draws it once onto a 2D canvas, and finally reads the color
of the four corners of the canvas to assert a specific value on them (within
some tolerance).

The test videos are designed with certain considerations in mind:

- They consist of a single still image so that the exact moment at which we
capture its pixels doesn't matter.
- We have cases where the video's visible size is the same as its coded size
(e.g., a 1280x720 H.264) and cases where it's not (e.g., a 640x360 H.264).
- In cases where the visible size is not the same as the coded size, the
non-visible area contains a color that's unexpected by the test so that certain
cases of incorrect cropping can be detected.
- We have a case where the visible rectangle does not start at (0, 0). These
videos are not expected to be common in the wild, but the H.264 specification
allows such exotic crops.
- The videos are long enough that we don't have to worry about the possibility
of capturing an empty frame when the video loops.

See [this](#generation-of-test-videos) for the script used to generate these
videos.

To run these tests use:

    tast run $HOST video.DrawOnCanvas.*

## Contents Tests (`video.Contents`)

This group of tests verifies that a video (presumably decoded using hardware
acceleration) is displayed correctly in full screen mode.

The tests start playing a video, switch it to full screen mode, take a
screenshot (using the CLI tool) and analyze the captured image to check the
color of a few interesting pixels. The test videos are re-used from the
[Canvas tests](#canvas-tests).

There are two variations: *_hw and *_composited_hw. The *_hw tests only run on
devices that support NV12 overlays (see `hwdep.SupportsNV12Overlays()`. The
*_composited_hw tests don't have this restriction: they force hardware overlays
off so that they have to be composited.

To check for color correctness, we sample a few interesting pixels. A video
frame should have the following structure:

```
*MMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMM*
M-MM-MMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMM-MM-M
MMMAAAAAAAAAAAAAAAAAAAAAAABBBBBBBBBBBBBBBBBBBBBBBMMM
M-MA-AAAAAAAAAAAAAAAAAAAAABBBBBBBBBBBBBBBBBBBBB-BM-M
MMMAAAAAAAAAAAAAAAAAAAAAAABBBBBBBBBBBBBBBBBBBBBBBMMM
MMMAAAAAAAAAAAAAAAAAAAAAAABBBBBBBBBBBBBBBBBBBBBBBMMM
MMMAAAAAAAAAAAAAAAAAAAAAAABBBBBBBBBBBBBBBBBBBBBBBMMM
MMMCCCCCCCCCCCCCCCCCCCCCCCDDDDDDDDDDDDDDDDDDDDDDDMMM
MMMCCCCCCCCCCCCCCCCCCCCCCCDDDDDDDDDDDDDDDDDDDDDDDMMM
MMMCCCCCCCCCCCCCCCCCCCCCCCDDDDDDDDDDDDDDDDDDDDDDDMMM
M-MC-CCCCCCCCCCCCCCCCCCCCCDDDDDDDDDDDDDDDDDDDDD-DM-M
MMMCCCCCCCCCCCCCCCCCCCCCCCDDDDDDDDDDDDDDDDDDDDDDDMMM
M-MM-MMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMM-MM-M
*MMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMM*
```

Where M is a magenta border; A, B, C, and D are the colors of the quadrants;
and - and * are the pixels we sample. * are the "outer corners" and - are the
"inner corners." Note that for the inner corners, we sample four pixels per
corner: three of those inside the magenta border and one inside the quadrant.
These pixels are near each other, so this helps us detect incorrect
stretching/shifting/rotation/mirroring. Sampling the outer corners helps us
detect leakage of invisible data.

Currently, these tests report the color distance for each sampled pixel as a
performance measurement. They don't currently fail due to unexpected colors.

To run these tests use:

    tast run $HOST video.Contents.*

## Addendum

### Generation of test videos (for `video.{DrawOnCanvas,Contents}`)

```
#!/bin/bash

# Generates an image of size canvas_width x canvas_height. The image contains an
# area of size area_width x area_height starting at (offset_x, offset_y) which
# has an 8px magenta interior border and the remaining area is subdivided into
# quadrants. Each quadrant is colored differently. The rest of the image is
# cyan. The colors are chosen so that the distance between any two colors is at
# least 127 (when taking the maximum absolute difference between components).
# That way, it's easy to programmatically distinguish among regions. Also, the
# assumption is that those colors are unlikely to correspond to common artifacts
# (e.g., green lines). The image is saved as output_file. An image of only
# area_width x area_height starting at (offset_x, offset_y) is saved as
# output_ref_file if provided: this is intended to be used as the "reference"
# image for how a video frame generated from output_file should be rendered in
# the absence of scaling and artifacts.
gen_image() {
  canvas_width=$1
  canvas_height=$2
  area_width=$3
  area_height=$4
  offset_x=$5
  offset_y=$6
  output_file=$7
  output_ref_file=$8

  # Calculate the coordinates of the top left corner of the area (x0, y0), the
  # middle of the area (x50, y50), and the bottom right corner (x100, y100).
  ((x0 = offset_x))
  ((y0 = offset_y))
  ((x50 = offset_x + area_width / 2 - 1))
  ((y50 = offset_y + area_height / 2 - 1))
  ((x100 = offset_x + area_width - 1))
  ((y100 = offset_y + area_height - 1))

  # Draw rectangles in the following order: top-left, top-right, bottom-right,
  # bottom-left.
  convert -size ${canvas_width}x${canvas_height} canvas:cyan -draw " \
    fill rgba(255, 0, 255) \
    rectangle ${x0},${y0} ${x100},${y100} \
    fill rgba(128, 128, 0, 255) \
    rectangle $((x0 + 8)),$((y0 + 8)) ${x50},${y50} \
    fill rgba(0, 128, 128, 255) \
    rectangle $((x50 + 1)),$((y0 + 8)) $((x100 - 8)),${y50} \
    fill rgba(128, 0, 128, 255) \
    rectangle $((x50 + 1)),$((y50 + 1)) $((x100 - 8)),$((y100 - 8)) \
    fill rgba(0, 0, 128, 255) \
    rectangle $((x0 + 8)),$((y50 + 1)) ${x50},$((y100 - 8))" \
    ${output_file}

  if [ -n "$output_ref_file" ]
  then
    convert ${output_file} -crop ${area_width}x${area_height}+${x0}+${y0} \
      +repage ${output_ref_file}
    exiftool -overwrite_original -all= ${output_ref_file}
  fi
}

# Given an image (input_file), generates an H.264 video which lasts 30 seconds,
# removes the EXIF metadata, and saves it as output_file.
gen_video() {
  input_file=$1
  output_file=$2
  ffmpeg -y -loop 1 -i ${input_file} -c:v libx264 -pix_fmt yuv420p -t 30 \
    -profile:v baseline ${output_file}
  exiftool -overwrite_original -all= ${output_file}
}

# Similar to gen_video(), but we get to specify the H.264 crop rectangle.
gen_cropped_video() {
  crop_top=$1
  crop_right=$2
  crop_bottom=$3
  crop_left=$4
  input_file=$5
  output_file=$6
  ffmpeg -y -loop 1 -i ${input_file} -c:v libx264 -pix_fmt yuv420p -t 30 \
    -profile:v baseline \
    -bsf:v h264_metadata="crop_top=${crop_top}: \
                          crop_right=${crop_right}: \
                          crop_bottom=${crop_bottom}: \
                          crop_left=${crop_left}" \
    ${output_file}
  exiftool -overwrite_original -all= ${output_file}
}

# Modifies file (MP4) to make sure the image width and source image width
# reported by exiftool is width. The offsets used here assume that the EXIF
# metadata has been removed from the file.
overwrite_image_width() {
  file=$1
  width=$2
  echo "000000f8: $(printf "%04x" ${width})" | xxd -r - ${file}
  echo "000001f1: $(printf "%04x" ${width})" | xxd -r - ${file}
}

# Same as overwrite_image_width() but for the height.
overwrite_image_height() {
  file=$1
  height=$2
  echo "000000fc: $(printf "%04x" ${height})" | xxd -r - ${file}
  echo "000001f3: $(printf "%04x" ${height})" | xxd -r - ${file}
}

################################################################################
# Generate typical videos:
#
# The visible rectangle for these videos starts at (0, 0). For some of these
# videos, the coded size is different from the visible size because the coded
# size is aligned to 16 on each dimension. For example, for a 640x360 H.264
# video, the coded size is 640x368. If we supplied a 640x360 image, ffmpeg will
# repeat the last row to fill the remaining 640x8 pixels prior to encoding. This
# is not desirable because if Chrome doesn't apply the visible rectangle for
# cropping (something that has occurred in the past) video.Contents and
# video.DrawOnCanvas will have a hard time detecting that regression because of
# the way the color at the edges are checked. Instead, we give ffmpeg a 640x368
# image where the last 640x8 are of a color not expected by the test. That way,
# ffmpeg doesn't have to pad. When we do this, we also have to tell ffmpeg the
# H.264 crop rectangle and modify the resulting MP4 file to make sure it carries
# a 640x360 size instead of 640x368.

gen_image 640 368 640 360 0 0 still-colors-360p.bmp \
  still-colors-360p.ref.png
gen_cropped_video 0 0 8 0 still-colors-360p.bmp still-colors-360p.h264.mp4
overwrite_image_height still-colors-360p.h264.mp4 360

gen_image 864 480 854 480 0 0 still-colors-480p.bmp \
  still-colors-480p.ref.png
gen_cropped_video 0 10 0 0 still-colors-480p.bmp still-colors-480p.h264.mp4
overwrite_image_width still-colors-480p.h264.mp4 854

gen_image 1280 720 1280 720 0 0 still-colors-720p.bmp \
  still-colors-720p.ref.png
gen_video still-colors-720p.bmp still-colors-720p.h264.mp4

gen_image 1920 1088 1920 1080 0 0 still-colors-1080p.bmp \
  still-colors-1080p.ref.png
gen_cropped_video 0 0 8 0 still-colors-1080p.bmp still-colors-1080p.h264.mp4
overwrite_image_height still-colors-1080p.h264.mp4 1080

################################################################################
# Generate a video with an exotic visible rectangle:
#
# H.264 allows for fancy visible rectangles that don't start at (0, 0). The
# video here is 720x480 but it is cropped by a different amount on each side
# (using H.264 metadata) in such a way that the visible area ends up being
# 640x360 (thus, the resulting video should be rendered essentially like
# still-colors-360p.h264.mp4 above).

gen_image 720 480 640 360 64 32 still-colors-720x480.bmp
gen_cropped_video 32 16 88 64 still-colors-720x480.bmp \
  still-colors-720x480-cropped-to-640x360.h264.mp4
```

[15-chipset-bdw-capabilities.yaml]: https://source.chromium.org/chromiumos/chromiumos/codesearch/+/master:src/overlays/chipset-bdw/chromeos-base/autotest-capability-chipset-bdw/files/15-chipset-bdw-capabilities.yaml?q=15-chipset-bdw-capabilities.yaml
[15-chipset-cml-capabilities.yaml]: https://source.chromium.org/chromiumos/chromiumos/codesearch/+/master:src/overlays/chipset-cml/chromeos-base/autotest-capability-chipset-cml/files/15-chipset-cml-capabilities.yaml?q=15-chipset-cml-capabilities.yaml
[`autotest-capability`]: https://chromium.googlesource.com/chromiumos/overlays/chromiumos-overlay/+/master/chromeos-base/autotest-capability-default/
[`caps` package]: https://source.chromium.org/chromiumos/chromiumos/codesearch/+/master:src/platform/tast-tests/src/chromiumos/tast/local/media/caps/caps.go
[tast video folder]: https://chromium.googlesource.com/chromiumos/platform/tast-tests/+/refs/heads/master/src/chromiumos/tast/local/bundles/cros/video/
[video_decode_accelerator_tests]: https://cs.chromium.org/chromium/src/media/gpu/video_decode_accelerator_tests.cc
[video decoder tests usage documentation]: https://chromium.googlesource.com/chromium/src/+/master/docs/media/gpu/video_decoder_test_usage.md
[video_decode_accelerator_perf_tests]: https://cs.chromium.org/chromium/src/media/gpu/video_decode_accelerator_perf_tests.cc
[video decoder performance tests usage documentation]: https://chromium.googlesource.com/chromium/src/+/master/docs/media/gpu/video_decoder_perf_test_usage.md
[video_encode_accelerator_unittest]: https://cs.chromium.org/chromium/src/media/gpu/video_encode_accelerator_unittest.cc
[`Play.h264_hw`]: https://source.chromium.org/chromiumos/chromiumos/codesearch/+/master:src/platform/tast-tests/src/chromiumos/tast/local/bundles/cros/video/play.go;l=92?q=h264_hw&ss=chromiumos%2Fchromiumos%2Fcodesearch
[`Play.vp8_hw`]: https://source.chromium.org/chromiumos/chromiumos/codesearch/+/master:src/platform/tast-tests/src/chromiumos/tast/local/bundles/cros/video/play.go;l=99?q=h264_hw&ss=chromiumos%2Fchromiumos%2Fcodesearch
[Test Dependencies]: https://chromium.googlesource.com/chromiumos/platform/tast/+/HEAD/docs/test_dependencies.md
[video-on-canvas.html]: https://source.chromium.org/chromiumos/chromiumos/codesearch/+/master:src/platform/tast-tests/src/chromiumos/tast/local/bundles/cros/video/data/video-on-canvas.html
[ffmpeg Slideshow docs]: https://trac.ffmpeg.org/wiki/Slideshow
[data/test_vectors/README.md]: https://source.chromium.org/chromiumos/chromiumos/codesearch/+/master:src/platform/tast-tests/src/chromiumos/tast/local/bundles/cros/video/data/test_vectors/README.md
