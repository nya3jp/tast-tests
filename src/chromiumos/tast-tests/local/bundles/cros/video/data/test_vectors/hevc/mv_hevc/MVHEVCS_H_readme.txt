MVHEVCS_H

Specification: All slices are coded as I, P or B slices. Only the first picture of each view is coded as IDR picture and thus an IRAP access unit. In addition, every two GOPs start with an access unit which contains pictures that are all CRA pictures. Each picture contains only one slice. NumViews is equal to 3. NumDirectRefLayers of the first non-base view is equal to 1 and NumDirectRefLayers of the second non-base view is equal to 2. For each picture in the non-base view, inter-view prediction is enabled. The three views are with the same spatial resolution. All NAL units are encapsulated into the byte stream format specified in ITU-T Rec. H.265 | ISO/IEC 23008-2.
Functional stage: Decoding of two views with inter-view prediction and inter-prediction.
Purpose: To conform the random access hierarchical B case (H random access 3-view case, IBP cfg.).

Software version : HTM-15.1
Number of coded frames : 50
Inter-view pred structure : I-B-P
Coded Sequence : Shark
QP : 35
Frame rate : 30
GOP size : 8
Intra period : 16 (Every two GOPs start with an access unit)
Level : 5.1
