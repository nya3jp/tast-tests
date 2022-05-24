WPP_F_ericsson_MAIN_2

Conformance point: HM-10.1

Picture size: 192x240

Frame rate: 25 frames/second

Specification:
entropy_coding_sync_enabled_flag is set to 1. A CTU size of 64x64 is used to encode the sequence. The picture is 3 CTUs wide. The sequence contains six GOPs, which are all 8 pictures long. The GOPs are encoded with varying numbers of slices and slice segments. Even frames have fixed QP, while the QP in odd frames is set so that abs(QP - QPSlice)>2.

The first GOP is coded using all Intra CUs, the second GOP is coded with skip disabled.

Random amounts of slice extension bytes are encoded in each slice header.


Functional stage:
Tests that entropy coding is correctly synchronised when a picture is 3 CTUs wide. Tests that the QP predictor is reset to QPslice at the beginning of every CTU row. May be used to test handling of entry points by a parallel decoder.


Purpose:
The bitstream checks that a decoder can correctly perform entropy coding synchronisation when a picture is 3 CTUs wide. It checks that a decoder can correctly derive QP predictors when entropy_coding_sync_enabled_flag is set to 1. It can also be used to check that a decoder can correctly handle entry points when slice header extensions are used.

