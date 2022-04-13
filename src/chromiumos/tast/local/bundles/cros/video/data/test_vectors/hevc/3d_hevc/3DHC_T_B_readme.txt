Name: 3DHC_T_B_Mediatek
Specification: The bitstream is coded with random access configuration as well as 3-view configuration. In addition to inter-view motion prediction, sub-PU inter-view motion predictionis enabled by setting log2_ivmc_sub_pb_size_minus3 less than 3 in the non-base texture views.
Functional stage: Decoding of three views with sub-PU inter-view motion prediction.
Purpose: To conform the sub-PU inter-view motion prediction.
Contributor: MediaTek.

Software: HTM-15.1
Configure(cfg file): baseCfg_3view+depth.cfg with interCompPred disabled in RPS + qpCfg_Nview+depth_QP30.cfg + seqCfg_Newspaper.cfg
IvMvPredFlag                        : 1 0
Configure(command line option): --FramesToBeEncoded 50 --Log2SubPbSizeMinus3=0 --IvResPredFlag=0 --IlluCompEnable=0 --ViewSynthesisPredFlag=0\
 --DepthRefinementFlag=0 --DepthBasedBlkPartFlag=0 --QtPredFlag=0 --QTL=0\
 --MpiFlag=0 --IvMvScalingFlag=0\
 --IntraContourFlag=0 --IntraWedgeFlag=0 --IntraSdcFlag=0 --DLT=0 --DepthIntraSkip=0 --InterSdcFlag=0
