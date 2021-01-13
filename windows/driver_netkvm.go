package windows

var driverNetKVM = DriverInfo{
	PackageName: "netkvm.inf_amd64_805ee20efb26a964",
	DriversRegistry: `Windows Registry Editor Version 5.00

[\DriverDatabase\DeviceIds\pci\VEN_1AF4&DEV_1000]
"{{ infFile }}"=hex(3):02,ff,00,00

[\DriverDatabase\DeviceIds\pci\VEN_1AF4&DEV_1000&SUBSYS_00011AF4&REV_00]
"{{ infFile }}"=hex(3):01,ff,00,00

[\DriverDatabase\DeviceIds\pci\VEN_1AF4&DEV_1041]
"{{ infFile }}"=hex(3):02,ff,00,00

[\DriverDatabase\DeviceIds\pci\VEN_1AF4&DEV_1041&SUBSYS_11001AF4&REV_01]
"{{ infFile }}"=hex(3):01,ff,00,00

[\DriverDatabase\DeviceIds\{{ classGuid|lower }}]
"{{ infFile }}"=hex(0):

[\DriverDatabase\DriverInfFiles\{{ infFile }}]
@=hex(7):{{ packageName|toHex }},00,00,00,00
"Active"=hex(1):{{ packageName|toHex }},00,00

[\DriverDatabase\DriverPackages\{{ packageName }}]
@=hex(1):{{ infFile|toHex }},00,00
"Catalog"=hex(1):6e,00,65,00,74,00,6b,00,76,00,6d,00,2e,00,63,00,61,00,74,00,00,00
"ImportDate"=hex(3):40,8f,c3,dd,bd,e9,d6,01
"InfName"=hex(1):6e,00,65,00,74,00,6b,00,76,00,6d,00,2e,00,69,00,6e,00,66,00,00,00
"OemPath"=hex(1):45,00,3a,00,5c,00,4e,00,65,00,74,00,4b,00,56,00,4d,00,5c,00,77,00,31,00,30,00,5c,00,61,00,6d,00,64,00,36,00,34,00,00,00
"Provider"=hex(1):52,00,65,00,64,00,20,00,48,00,61,00,74,00,2c,00,20,00,49,00,6e,00,63,00,2e,00,00,00
"SignerName"=hex(1):00,00
"SignerScore"=dword:0d000004
"StatusFlags"=dword:00000012
"Version"=hex(3):00,ff,09,00,00,00,00,00,72,e9,36,4d,25,e3,ce,11,bf,c1,08,00,2b,e1,03,18,00,00,8e,c3,86,b8,d6,01,38,4a,68,00,53,00,64,00,00,00,00,00,00,00,00,00
`,
}
