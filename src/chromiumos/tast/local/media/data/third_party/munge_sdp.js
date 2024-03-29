/**
 * Copyright 2016 The Chromium Authors
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

// This file was copied from:
// https://cs.chromium.org/chromium/src/chrome/test/data/webrtc/munge_sdp.js.

/**
 * See |setSdpDefaultCodec|.
 */
function setSdpDefaultVideoCodec(sdp, codec) {
  return setSdpDefaultCodec(sdp, 'video', codec);
}

/**
 * Returns a modified version of |sdp| where the |codec| has been promoted to be
 * the default codec, i.e. the codec whose ID is first in the list of codecs on
 * the 'm=|type|' line, where |type| is 'audio' or 'video'.
 * @private
 */
function setSdpDefaultCodec(sdp, type, codec) {
  var sdpLines = splitSdpLines(sdp);

  // Find codec ID, e.g. 100 for 'VP8' if 'a=rtpmap:100 VP8/9000'.
  var codecId = findRtpmapId(sdpLines, codec);
  if (codecId === null) {
    failure('sdpPreferCodec',
            'Missing a=rtpmap entry for |codec| = ' + codec + ' in ' + sdp);
  }

  // Find 'm=|type|' line, e.g. 'm=video 9 UDP/TLS/RTP/SAVPF 100 101 107 116'.
  var mLineNo = findLine(sdpLines, 'm=' + type);
  if (mLineNo === null) {
    failure('setSdpDefaultCodec',
             '\'m=' + type + '\' line missing from |sdp|.');
  }

  // Modify video line to use the desired codec as the default.
  sdpLines[mLineNo] = setMLineDefaultCodec(sdpLines[mLineNo], codecId);
  return mergeSdpLines(sdpLines);
}

/**
 *  * See |getSdpDefaultCodec|.
 *   */
function getSdpDefaultVideoCodec(sdp) {
  return getSdpDefaultCodec(sdp, 'video');
}

/**
 * Gets the default codec according to the |sdp|, i.e. the name of the codec
 * whose ID is first in the list of codecs on the 'm=|type|' line, where |type|
 * is 'audio' or 'video'.
 * @private
 */
function getSdpDefaultCodec(sdp, type) {
  var sdpLines = splitSdpLines(sdp);

  // Find 'm=|type|' line, e.g. 'm=video 9 UDP/TLS/RTP/SAVPF 100 101 107 116'.
  var mLineNo = findLine(sdpLines, 'm=' + type);
  if (mLineNo === null) {
    failure('getSdpDefaultCodec',
             '\'m=' + type + '\' line missing from |sdp|.');
  }

  // The default codec's ID.
  var defaultCodecId = getMLineDefaultCodec(sdpLines[mLineNo]);
  if (defaultCodecId === null) {
    failure('getSdpDefaultCodec',
             '\'m=' + type + '\' line contains no codecs.');
  }

  // Find codec name, e.g. 'VP8' for 100 if 'a=rtpmap:100 VP8/9000'.
  var defaultCodec = findRtpmapCodec(sdpLines, defaultCodecId);
  if (defaultCodec === null) {
    failure('getSdpDefaultCodec',
             'Unknown codec name for default codec ' + defaultCodecId + '.');
  }
  return defaultCodec;
}

/**
 * Searches through all |sdpLines| for the 'a=rtpmap:' line for the codec of
 * the specified name, returning its ID as an int if found, or null otherwise.
 * |codec| is the case-sensitive name of the codec.
 * For example, if |sdpLines| contains 'a=rtpmap:100 VP8/9000' and |codec| is
 * 'VP8', this function returns 100.
 * @private
 */
function findRtpmapId(sdpLines, codec) {
  var lineNo = findRtpmapLine(sdpLines, codec);
  if (lineNo === null)
    return null;
  // Parse <id> from 'a=rtpmap:<id> <codec>/<rate>'.
  var id = sdpLines[lineNo].substring(9, sdpLines[lineNo].indexOf(' '));
  return parseInt(id);
}

/**
 * Searches through all |sdpLines| for the 'a=rtpmap:' line for the codec of
 * the specified codec ID, returning its name if found, or null otherwise.
 * For example, if |sdpLines| contains 'a=rtpmap:100 VP8/9000' and |id| is 100,
 * this function returns 'VP8'.
 * @private
 */
function findRtpmapCodec(sdpLines, id) {
  var lineNo = findRtpmapLine(sdpLines, id);
  if (lineNo === null)
    return null;
  // Parse <codec> from 'a=rtpmap:<id> <codec>/<rate>'.
  var from = sdpLines[lineNo].indexOf(' ');
  var to = sdpLines[lineNo].indexOf('/', from);
  if (from === null || to === null || from + 1 >= to)
    failure('findRtpmapCodec', '');
  return sdpLines[lineNo].substring(from + 1, to);
}

/**
 * Finds the first 'a=rtpmap:' line from |sdpLines| that contains |contains| and
 * returns its line index, or null if no such line was found. |contains| may be
 * the codec ID, codec name or bitrate. An 'a=rtpmap:' line looks like this:
 * 'a=rtpmap:<id> <codec>/<rate>'.
 */
function findRtpmapLine(sdpLines, contains) {
  for (var i = 0; i < sdpLines.length; i++) {
    // Is 'a=rtpmap:' line containing |contains| string?
    if (sdpLines[i].startsWith('a=rtpmap:') &&
        sdpLines[i].indexOf(contains) != -1) {
      // Expecting pattern 'a=rtpmap:<id> <codec>/<rate>'.
      var pattern = new RegExp('a=rtpmap:(\\d+) \\w+\\/\\d+');
      if (!sdpLines[i].match(pattern))
        failure('findRtpmapLine', 'Unexpected "a=rtpmap:" pattern.');
      // Return line index.
      return i;
    }
  }
  return null;
}

/**
 * Returns a modified version of |mLine| that has |codecId| first in the list of
 * codec IDs. For example, setMLineDefaultCodec(
 *     'm=video 9 UDP/TLS/RTP/SAVPF 100 101 107 116 117 96', 107)
 * Returns:
 *     'm=video 9 UDP/TLS/RTP/SAVPF 107 100 101 116 117 96'
 * @private
 */
function setMLineDefaultCodec(mLine, codecId) {
  var elements = mLine.split(' ');

  // Copy first three elements, codec order starts on fourth.
  var newLine = elements.slice(0, 3);

  // Put target |codecId| first and copy the rest.
  newLine.push(codecId);
  for (var i = 3; i < elements.length; i++) {
    if (elements[i] != codecId)
      newLine.push(elements[i]);
  }

  return newLine.join(' ');
}

/**
 * Returns the default codec's ID from the |mLine|, or null if the codec list is
 * empty. The default codec is the codec whose ID is first in the list of codec
 * IDs on the |mLine|. For example, getMLineDefaultCodec(
 *     'm=video 9 UDP/TLS/RTP/SAVPF 100 101 107 116 117 96')
 * Returns:
 *     100
 * @private
 */
function getMLineDefaultCodec(mLine) {
  var elements = mLine.split(' ');
  if (elements.length < 4)
    return null;
  return parseInt(elements[3]);
}

/** @private */
function splitSdpLines(sdp) {
  return sdp.split('\r\n');
}

/** @private */
function mergeSdpLines(sdpLines) {
  return sdpLines.join('\r\n');
}

/** @private */
function findLine(lines, startsWith) {
  for (var i = 0; i < lines.length; i++) {
    if (lines[i].startsWith(startsWith))
      return i;
  }
  return null;
}
