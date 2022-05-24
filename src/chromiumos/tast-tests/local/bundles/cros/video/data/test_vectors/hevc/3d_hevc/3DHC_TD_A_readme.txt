Test bitstream #3DHC_TD_A

Specification: The bitstream is coded with random access configuration as well as 3-view configuration. In addition to all the texture coding tools, all the depth coding tools and depth dependent texture coding tools, sub-PU MPI (Motion Parameter Inheritance) are turned on.
Functional stage: Decoding of three views with sub-PU MPI enabled for depth views.
Purpose: To conform the sub-PU MPI for depth.

resolution:     1024x768 (texture:depth=1:1)
# of views:     3T+3D
# of frames:    48
Coding Structure(time): IBP
Coding Structure(view): PIP
Bitstream name: 3DHC_TD_A.hevc

Software: HTM15.1

Configurations:
#========== depth coding tools ==========
VSO                                 : 1                 # use of view synthesis optimization for depth coding
IntraWedgeFlag                      : 1
IntraContourFlag                    : 0                 # use of intra-view prediction mode
IntraSdcFlag                        : 1
DLT                                 : 1
QTL                                 : 0
QtPredFlag                          : 0
InterSdcFlag                        : 1                             # use of inter sdc
MpiFlag                             : 1
IntraSingleFlag                     : 1                 # use of single depth mode