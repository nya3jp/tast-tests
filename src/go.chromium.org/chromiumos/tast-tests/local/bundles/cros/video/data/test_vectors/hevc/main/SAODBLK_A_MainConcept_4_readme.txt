file_name: SAODBLK_A_MainConcept_4.hevc
resolution & level: 1016x760, main level 4.1
frame-rate: 29.97
length: 500 frames
features:
The unique features of these streams are 1) non-rectangular shape of slices and 2) the minimum size of CTU, which for chroma planes takes the value of 8x8 and exactly coincides with a block that is used for deblocking. These unique features together with SAO enabled impose a lot of challenges in multithreaded decoding for correct implementation of loop filtering on slice borders and especially slice convex and concave corners.
SAODBLK_A_Divx_1.bin has only slices
SliceMode                : 1
SliceArgument            : 128
SliceSegmentMode         : 1
SliceSegmentArgument     : 128
LFCrossSliceBoundaryFlag : 1