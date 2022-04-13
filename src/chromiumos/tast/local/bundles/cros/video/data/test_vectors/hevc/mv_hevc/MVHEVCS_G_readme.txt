Test bitstream #MVHEVCS-G

Specification: All slices are coded as I, P or B slices. Only the first picture of each view is coded as an IDR picture. Thus the first access unit is an IRAP access unit. In addition, every two GOPs start with an access unit which contains pictures that are all CRA pictures. Each picture contains only one slice. NumViews is equal to 3. NumDirectRefLayers of the non-base view is equal to 1. For each picture in the non-base view, inter-view prediction is enabled. The three views are with the same spatial resolution. All NAL units are encapsulated into the byte stream format specified in Rec. ITU-T H.265 | ISO/IEC 23008-2.
Functional stage: Decoding of three views with inter-view prediction and inter-prediction.
Purpose: To conform the random access hierarchical B case (random access 3-view case, PIP configuration: wherein the middle view is the base view and each of the left and right views depends only on the base view).

resolution:     1024x768
# of views:     3 Texture
# of frames:    49
Coding Structure(time): IBP
Coding Structure(view): PIP
Bitstream name: MVHEVCS_G.hevc

Software: HTM-16.1

Configurations:
#======== Coding Structure =============
IntraPeriod                   : 25          # Period of I-Frame ( -1 = only first)
DispSearchRangeRestriction    : 1           # Limit Search range for vertical component of disparity vector