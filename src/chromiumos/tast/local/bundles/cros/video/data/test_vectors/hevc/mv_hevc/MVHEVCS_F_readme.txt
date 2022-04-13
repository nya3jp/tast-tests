Test bitstream #MVHEVCS-F

Specification: All slices are coded as I, P or B slices. Only the first picture of each view is coded as IDR picture and thus an IRAP access unit. In addition, for each of every two following GOPs, an access unit which contains pictures that are all CRA pictures are requested. Other pictures are non-IRAP pictures. Each picture contains only one slice. NumViews is equal to 2. NumActiveRefLayerPics is equal to 1 for each picture in an IRAP access unit of the non-base view, and 0 otherwise, i.e., only for each picture in the non-base view in the IRAP access unit, inter-view prediction is enabled. The two views are with the same spatial resolution. All NAL units are encapsulated into the byte stream format specified in ITU-T Rec. H.265Rec. ITU-T H.265 | ISO/IEC 23008-2.
Functional stage: Decoding of two views with inter-view prediction only for random access points.
Purpose: To conform the flexibility of inter-view prediction applicability scope (inter-view pred. only for IRAP pictures).

resolution:     1024x768
# of views:     2 Texture
# of frames:    48
Coding Structure(time): IBP
Coding Structure(view): PI
Bitstream name: MVHEVCS_F.hevc

Software: HTM15.1

Configurations:
#======== Coding Structure =============
IntraPeriod                   : 16          # Period of I-Frame ( -1 = only first)
DispSearchRangeRestriction    : 1           # Limit Search range for vertical component of disparity vector
DecodingRefreshType           : 1           # Random Accesss 0:none, 1:CDR, 2:IDR
GOPSize                       : 8           # GOP Size (number of B slice = GOPSize-1)

#                           QPfactor      betaOffsetDiv2   #ref_pics_active  reference pictures     deltaRPS     reference idcs          ilPredLayerIdc       refLayerPicPosIl_L1
#         Type  POC QPoffset     tcOffsetDiv2      temporal_id      #ref_pics                 predict     #ref_idcs        #ActiveRefLayerPics     refLayerPicPosIl_L0
Frame1:     B    8     1     0.442    0        0        0        4      4     -8 -10 -12 -16     0      0
Frame2:     B    4     2     0.3536   0        0        0        2      3     -4 -6  4           1      4    5     1 1 0 0 1         0
Frame3:     B    2     3     0.3536   0        0        0        2      4     -2 -4  2 6         1      2    4     1 1 1 1           0
Frame4:     B    1     4     0.68     0        0        0        2      4     -1  1  3 7         1      1    5     1 0 1 1 1         0
Frame5:     B    3     4     0.68     0        0        0        2      4     -1 -3  1 5         1     -2    5     1 1 1 1 0         0
Frame6:     B    6     3     0.3536   0        0        0        2      4     -2 -4 -6 2         1     -3    5     1 1 1 1 0         0
Frame7:     B    5     4     0.68     0        0        0        2      4     -1 -5  1 3         1      1    5     1 0 1 1 1         0
Frame8:     B    7     4     0.68     0        0        0        2      4     -1 -3 -7 1         1     -2    5     1 1 1 1 0         0

FrameI_l1:  P    0     3     0.442    0        0        0        1      0                        0                                   1          0          0           -1
Frame1_l1:  B    8     4     0.442    0        0        0        4      4     -8 -10 -12 -16     0                                   0          0          -1          -1
Frame2_l1:  B    4     5     0.3536   0        0        0        3      3     -4 -6  4           1     4     5     1 1 0 0 1         0          0          -1          -1
Frame3_l1:  B    2     6     0.3536   0        0        0        3      4     -2 -4  2 6         1     2     4     1 1 1 1           0          0          -1          -1
Frame4_l1:  B    1     7     0.68     0        0        0        3      4     -1  1  3 7         1     1     5     1 0 1 1 1         0          0          -1          -1
Frame5_l1:  B    3     7     0.68     0        0        0        3      4     -1 -3  1 5         1    -2     5     1 1 1 1 0         0          0          -1          -1
Frame6_l1:  B    6     6     0.3536   0        0        0        3      4     -2 -4 -6 2         1    -3     5     1 1 1 1 0         0          0          -1          -1
Frame7_l1:  B    5     7     0.68     0        0        0        3      4     -1 -5  1 3         1     1     5     1 0 1 1 1         0          0          -1          -1
Frame8_l1:  B    7     7     0.68     0        0        0        3      4     -1 -3 -7 1         1    -2     5     1 1 1 1 0         0          0          -1          -1
