Test bitstream #3DHC_T_A

Specification: The bitstream is coded with random access configuration as well as 3-view configuration. In addition to inter-view motion prediction, advanced residual prediction is enabled in the non-base texture views.
Functional stage: Decoding of three views with inter-view motion prediction and advanced residual prediction (ARP).
Purpose: To conform the ARP.

resolution:     1024x768
# of views:     3T+3D
# of frames:    48
Coding Structure(time): IBP
Coding Structure(view): PIP
Bitstream name: 3DHC_T_A.hevc



Software: HTM15.1

Configurations:
#========== multiview coding tools ==========
IvMvPredFlag                        : 1 0              # Inter-view motion prediction
IvResPredFlag                       : 1                # Advanced inter-view residual prediction (0:off, 1:on)
IlluCompEnable                      : 0                # Enable Illumination compensation ( 0: off, 1: on )  (v/d)
IlluCompLowLatencyEnc               : 0                # Enable low-latency Illumination compensation encoding( 0: off, 1: on )
ViewSynthesisPredFlag               : 0                # View synthesis prediction
DepthRefinementFlag                 : 0                # Disparity refined by depth DoNBDV
IvMvScalingFlag                     : 1                # Interview motion vector scaling
Log2SubPbSizeMinus3                 : 3                # Log2 of sub-PU size minus 3 for IvMvPred (0 ... 3) and smaller than or equal to log2(maxCUSize)-3
Log2MpiSubPbSizeMinus3              : 3                # Log2 of sub-PU size minus 3 for MPI (0 ... 3) and smaller than or equal to log2(maxCUSize)-3
DepthBasedBlkPartFlag               : 0                # Depth-based Block Partitioning

#========== depth coding tools ==========
VSO                                 : 0                 # use of view synthesis optimization for depth coding
IntraWedgeFlag                      : 0
IntraContourFlag                    : 0                 # use of intra-view prediction mode
IntraSdcFlag                        : 0
DLT                                 : 0
QTL                                 : 0
QtPredFlag                          : 0
InterSdcFlag                        : 0                             # use of inter sdc
MpiFlag                             : 0
DepthIntraSkip                      : 0