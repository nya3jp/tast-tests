// Copyright 2021 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

class MP4Source {
  constructor(uri) {
    this.file = MP4Box.createFile();
    this.file.onError = console.error.bind(console);
    this.file.onReady = this.onReady.bind(this);
    this.file.onSamples = this.onSamples.bind(this);

    this.uri = uri;
    this.info = null;
    this._info_resolver = null;
  }

  async initialize() {
    let response = await fetch(this.uri);

    const reader = response.body.getReader();
    let offset = 0;
    let mp4File = this.file;

    function appendBuffers({done, value}) {
      if(done) {
        mp4File.flush();
        return;
      }

      let buf = value.buffer;
      buf.fileStart = offset;

      offset += buf.byteLength;

      mp4File.appendBuffer(buf);

      return reader.read().then(appendBuffers);
    }

    return reader.read().then(appendBuffers);
  }

  onReady(info) {
    this.info = info;

    if (this._info_resolver) {
      this._info_resolver(info);
      this._info_resolver = null;
    }
  }

  getInfo() {
    if (this.info)
      return Promise.resolve(this.info);

    return new Promise((resolver) => { this._info_resolver = resolver; });
  }

  getAvccBox() {
    return this.file.moov.traks[0].mdia.minf.stbl.stsd.entries[0].avcC;
  }

  getVpccBox() {
    return this.file.moov.traks[0].mdia.minf.stbl.stsd.entries[0].vpcC;
  }

  getAv1cBox() {
    return this.file.moov.traks[0].mdia.minf.stbl.stsd.entries[0].av1C;
  }

  start(track, onChunk) {
    this._onChunk = onChunk;
    this.file.setExtractionOptions(track.id);
    this.file.start();
  }

  onSamples(track_id, ref, samples) {
    for (const sample of samples) {
      const type = sample.is_sync ? "key" : "delta";

      const chunk = new EncodedVideoChunk({
        type: type,
        timestamp: sample.cts,
        duration: sample.duration,
        data: sample.data
      });

      this._onChunk(chunk);
    }
  }
}

class Writer {
  constructor(size) {
    this.data = new Uint8Array(size);
    this.idx = 0;
    this.size = size;
  }

  getData() {
    if(this.idx != this.size)
      throw "Mismatch between size reserved and sized used"

    return this.data.slice(0, this.idx);
  }

  writeUint8(value) {
    this.data.set([value], this.idx);
    this.idx++;
  }

  writeUint16(value) {
    var arr = Uint16Array.of(value);
    var buffer = new Uint8Array(arr.buffer);
    this.data.set([buffer[1], buffer[0]], this.idx);
    this.idx += 2;
  }

  writeUint8Array(value) {
    this.data.set(value, this.idx);
    this.idx += value.length;
  }
}

class MP4Demuxer {
  constructor(uri) {
    this.source = new MP4Source(uri);
  }

  getVPxExtraData(vpccBox) {
    // See https://www.webmproject.org/vp9/mp4/#syntax_1 for detail.
    const vpxFixedExtraSize = 8;
    let size = vpxFixedExtraSize + vpccBox.codecIntializationData.length;
    var writer = new Writer(size);

    writer.writeUint8(vpccBox.profile);
    writer.writeUint8(vpccBox.level);
    writer.writeUint8(((vpccBox.bitDepth << 4) |
                       (vpccBox.chromaSubsampling << 1) |
                       vpccBox.videoFullRangeFlag));
    writer.writeUint8(vpccBox.colourPrimaries);
    writer.writeUint8(vpccBox.transferCharacteristics);
    writer.writeUint8(vpccBox.matrixCoefficients);
    writer.writeUint16(vpccBox.codecIntializationDataSize);

    for (let i = 0; i < vpccBox.codecIntializationData.length; i++) {
      writer.writeUint8(vpccBox.codecIntializationData[i]);
    }

    return writer.getData();
  }

  getAV1ExtraData(av1cBox) {
    // See https://aomediacodec.github.io/av1-isobmff/#av1codecconfigurationbox-syntax
    // for detail.
    const av1FixedExtraSize = 4;
    let size = av1FixedExtraSize + av1cBox.configOBUs.length;
    var writer = new Writer(size);

    const markerAndVersion = (0x1 << 7) | 0x1;
    writer.writeUint8(markerAndVersion);
    writer.writeUint8((av1cBox.seq_profile << 5) | (av1cBox.seq_level_idx_0));
    let value = av1cBox.high_bitdepth;
    value = (value << 1) + av1cBox.twelve_bit;
    value = (value << 1) + av1cBox.monochrome;
    value = (value << 1) + av1cBox.chroma_sabsampling_x;
    value = (value << 1) + av1cBox.chroma_sabsampling_y;
    value = (value << 1) + av1cBox.chroma_sample_position;
    writer.writeUint8(value);

    if (av1cBox.initial_presentation_delay_present) {
      writer.writeUint8(av1cBox.initial_presentation_delay_minus_one);
    } else {
      writer.writeUint8(0);
    }

    for (let i = 0; i < av1cBox.configOBUs.length; i++) {
      writer.writeUint8(av1cBox.configOBUs[i]);
    }

    return writer.getData();
  }

  getH264ExtraData(avccBox) {
    var i;
    var size = 7;
    for (i = 0; i < avccBox.SPS.length; i++) {
      // NALU length is encoded as a uint16.
      size += 2 + avccBox.SPS[i].length;
    }
    for (i = 0; i < avccBox.PPS.length; i++) {
      // NALU length is encoded as a uint16.
      size += 2 + avccBox.PPS[i].length;
    }

    var writer = new Writer(size);

    writer.writeUint8(avccBox.configurationVersion);
    writer.writeUint8(avccBox.AVCProfileIndication);
    writer.writeUint8(avccBox.profile_compatibility);
    writer.writeUint8(avccBox.AVCLevelIndication);
    writer.writeUint8(avccBox.lengthSizeMinusOne + (63<<2));

    writer.writeUint8(avccBox.nb_SPS_nalus + (7<<5));
    for (i = 0; i < avccBox.SPS.length; i++) {
      writer.writeUint16(avccBox.SPS[i].length);
      writer.writeUint8Array(avccBox.SPS[i].nalu);
      window.temp = avccBox.SPS[i].nalu;
    }

    writer.writeUint8(avccBox.nb_PPS_nalus);
    for (i = 0; i < avccBox.PPS.length; i++) {
      writer.writeUint16(avccBox.PPS[i].length);
      writer.writeUint8Array(avccBox.PPS[i].nalu);
    }

    return writer.getData();
  }

  async getConfig() {
    await this.source.initialize();
    let info = await this.source.getInfo();

    this.track = info.videoTracks[0];
    let codec = this.track.codec;
    let extradata;
    if (this.track.codec.startsWith("avc1")) {
      extradata = this.getH264ExtraData(this.source.getAvccBox());
    } else if (this.track.codec.startsWith("vp08")) {
      codec = "vp8";
      extradata = this.getVPxExtraData(this.source.getVpccBox());
    } else if (this.track.codec.startsWith("vp09")) {
      extradata = this.getVPxExtraData(this.source.getVpccBox());
    } else if (this.track.codec.startsWith("av01")) {
      extradata = this.getAV1ExtraData(this.source.getAv1cBox());
    }

    let config = {
      codec: codec,
      codedHeight: this.track.track_height,
      codedWidth: this.track.track_width,
      description: extradata,
    }

    return Promise.resolve(config);
  }

  start(onChunk) {
    this.source.start(this.track, onChunk);
  }
}
