RAP_A_docomo_6: (RAP_A_docomo_6.hevc)
Frame rate: 30 fps
Picture size: 832x480
Spec version: HM10.1

(Category: RAP; Sub-category: Bitstream starting with a CRA picture followed by RASL pictures that cannot be decoded)

The purpose of the stream is to exercise the decoding of a conforming bitstream where the CRA is the first picture in the bitstream and is followed by 7 RASL pictures that are not decodable. There are two subsequent CRA pictures with RASL pictures, following the first CRA picture in this bitstream. These subsequent RASL pictures should be decodable since the associated CRA is not the first CRA picture in the bitstream.

Note: In actual decoders, any RASL pictures associated with a CRA picture at the beginning of the bitstream or any RASL pictures associated with a BLA picture may be ignored (removed from the bitstream and discarded), as they are not specified for output and have no effect on the decoding process of any other pictures that are specified for output.

The MD5 of the yuv file in output order decoded using the HM10.1-dev-3420 is included in RAP_A_docomo_6.md5.


