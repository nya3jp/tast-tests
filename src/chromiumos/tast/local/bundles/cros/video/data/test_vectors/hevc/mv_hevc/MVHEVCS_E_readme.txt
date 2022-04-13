Test bitstream #MVHEVCS-E

Specification: All slices are coded as I, P or B slices. Only the first picture of each view is coded as IDR picture and thus an IRAP access unit. In addition, every two GOPs start with an access unit which contains pictures that are all CRA pictures. Each picture contains only one slice. NumViews is equal to 2. NumDirectRefLayers of the non-base view is equal to 1. For each picture in the non-base view, inter-view prediction is enabled. The two views are with the same spatial resolution. All NAL units are encapsulated into the byte stream format specified in ITU-T Rec. H.265Rec. ITU-T H.265 | ISO/IEC 23008-2.
Functional stage: Decoding of two views with inter-view prediction and inter-prediction.
Purpose: To conform the random access hierarchical B case.

resolution:     1024x768
# of views:     2 Texture
# of frames:    48
Coding Structure(time): IBP
Coding Structure(view): PI
Bitstream name: MVHEVCS_E.hevc

Software: HTM15.1

Configurations:
#======== Coding Structure =============
IntraPeriod                   : 16          # Period of I-Frame ( -1 = only first)
DispSearchRangeRestriction    : 1           # Limit Search range for vertical component of disparity vector