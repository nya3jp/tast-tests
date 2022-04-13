WPP_B_ericsson_MAIN_2

Conformance point: HM-10.1

Picture size: 416x240

Frame rate: 25 frames/second

Specification:
entropy_coding_sync_enabled_flag is set to 1. A CTU size of 32x32 is used to encode the sequence. The sequence contains six GOPs, which are all 8 pictures long. The first three GOPs have pictures with the following characteristics:
1.	One slice in the frame, constant QP
2.	One slice in the frame, QP of each TU is set to a value such that abs(QP - QPSlice)>2.
3.	Maximum number of independent slice segments in the frame, at least one slice segment is 1 CTU long, at least one slice segment is 2 CTUs long, constant QP
4.	Maximum number of independent slice segments in the frame, at least one slice segment is 1 CTU long, at least one slice segment is 2 CTUs long,   QP of each TU is set to a value such that abs(QP - QPSlice)>2.
5.	Maximum number of dependent slice segments in the frame, at least one dependent slice segment is 1 CTU long, at least one dependent slice segment is 2 CTUs long, constant QP
6.	Maximum number of dependent slice segments in the frame, at least one dependent slice segment is 1 CTU long, at least one dependent slice segment is 2 CTUs long, QP of each TU is set to a value such that abs(QP - QPSlice)>2.
7.	Random combination of independent/dependent slice segments, constant QP
8.	Random combination of independent/dependent slice segments, QP of each TU is set to a value such that abs(QP - QPSlice)>2.

The first GOP is coded using all Intra CUs, the second GOP is coded with skip disabled.

The final three GOPs feature a mixture of single slice pictures, and pictures coded using multiple slices and multiple slice segments.

Random amounts of slice extension bytes are encoded in each slice header.


Functional stage:
Tests that entropy coding is correctly synchronised. Tests that the QP predictor is reset to QPslice at the beginning of every CTU row. May be used to test handling of entry points by a parallel decoder.


Purpose:
The bitstream checks that a decoder can correctly perform entropy coding synchronisation with and without different types of slice. It checks that a decoder can correctly derive QP predictors when entropy_coding_sync_enabled_flag is set to 1. It can also be used to check that a decoder can correctly handle entry points when slice header extensions are used.

