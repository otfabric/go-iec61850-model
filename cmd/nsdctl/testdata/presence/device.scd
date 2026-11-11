<?xml version="1.0" encoding="UTF-8"?>
<!-- Synthetic SCL written for nsdctl tests (OT Fabric, MIT). -->
<SCL xmlns="http://www.iec.ch/61850/2003/SCL" version="2007" revision="B">
  <IED name="TEST">
    <AccessPoint name="AP1">
      <Server>
        <LDevice inst="CTRL">
          <LN0 lnClass="LLN0" inst="" lnType="T_LLN0">
            <DOI name="NamPlt"><DAI name="ldNs"><Val>IEC 61850-7-4:2007B</Val></DAI></DOI>
          </LN0>
          <LN prefix="Q0" lnClass="XCBR" inst="1" lnType="T_XCBR">
            <DOI name="Pos"><DAI name="ctlModel"><Val>direct-with-normal-security</Val></DAI></DOI>
          </LN>
          <LN prefix="Q1" lnClass="XCBR" inst="1" lnType="T_XCBR"/>
        </LDevice>
      </Server>
    </AccessPoint>
  </IED>
  <DataTypeTemplates>
    <LNodeType id="T_LLN0" lnClass="LLN0">
      <DO name="Beh" type="T_ENS"/>
      <DO name="NamPlt" type="T_LPL"/>
    </LNodeType>
    <LNodeType id="T_XCBR" lnClass="XCBR">
      <DO name="Beh" type="T_ENS"/>
      <DO name="Loc" type="T_SPS"/>
      <DO name="OpCnt" type="T_INS"/>
      <DO name="Pos" type="T_DPC"/>
      <DO name="BlkOpn" type="T_SPC"/>
      <DO name="BlkCls" type="T_SPC"/>
      <DO name="CBOpCap" type="T_ENS"/>
    </LNodeType>
    <DOType id="T_ENS" cdc="ENS">
      <DA name="stVal" bType="Enum" type="X" fc="ST"/>
      <DA name="q" bType="Quality" fc="ST"/>
      <DA name="t" bType="Timestamp" fc="ST"/>
    </DOType>
    <DOType id="T_SPS" cdc="SPS">
      <DA name="stVal" bType="BOOLEAN" fc="ST"/>
      <DA name="q" bType="Quality" fc="ST"/>
      <DA name="t" bType="Timestamp" fc="ST"/>
    </DOType>
    <DOType id="T_INS" cdc="INS">
      <DA name="stVal" bType="INT32" fc="ST"/>
      <DA name="q" bType="Quality" fc="ST"/>
      <DA name="t" bType="Timestamp" fc="ST"/>
    </DOType>
    <DOType id="T_LPL" cdc="LPL">
      <DA name="vendor" bType="VisString255" fc="DC"/>
      <DA name="swRev" bType="VisString255" fc="DC"/>
      <DA name="ldNs" bType="VisString255" fc="EX"/>
      <DA name="configRev" bType="VisString255" fc="DC"/>
    </DOType>
    <DOType id="T_DPC" cdc="DPC">
      <DA name="origin" bType="Struct" type="T_Originator" fc="ST"/>
      <DA name="stVal" bType="Dbpos" fc="ST"/>
      <DA name="q" bType="Quality" fc="ST"/>
      <DA name="t" bType="Timestamp" fc="ST"/>
      <DA name="ctlModel" bType="Enum" type="CtlModelKind" fc="CF"><Val>sbo-with-normal-security</Val></DA>
    </DOType>
    <DOType id="T_SPC" cdc="SPC">
      <DA name="stVal" bType="BOOLEAN" fc="ST"/>
      <DA name="q" bType="Quality" fc="ST"/>
      <DA name="t" bType="Timestamp" fc="ST"/>
      <DA name="ctlModel" bType="Enum" type="CtlModelKind" fc="CF"><Val>status-only</Val></DA>
    </DOType>
    <DAType id="T_Originator">
      <BDA name="orCat" bType="Enum" type="OrCat"/>
      <BDA name="orIdent" bType="Octet64"/>
    </DAType>
  </DataTypeTemplates>
</SCL>
