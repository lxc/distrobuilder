package windows

var driverBalloon = DriverInfo{
	PackageName: "balloon.inf_amd64_c6bc3e0b232c3c2d",
	SoftwareRegistry: `Windows Registry Editor Version 5.00

[\Microsoft\Windows\CurrentVersion\Setup\PnpLockdownFiles\%SystemRoot%/System32/drivers/balloon.sys]
"Class"=dword:00000004
"Owners"=hex(7):{{ infName }},00,00,00,00
"Source"=hex(2):25,00,53,00,79,00,73,00,74,00,65,00,6d,00,52,00,6f,00,6f,00,74,00,25,00,5c,00,53,00,79,00,73,00,74,00,65,00,6d,00,33,00,32,00,5c,00,44,00,72,00,69,00,76,00,65,00,72,00,53,00,74,00,6f,00,72,00,65,00,5c,00,46,00,69,00,6c,00,65,00,52,00,65,00,70,00,6f,00,73,00,69,00,74,00,6f,00,72,00,79,00,5c,00,{{ packageName|toHex }},5c,00,62,00,61,00,6c,00,6c,00,6f,00,6f,00,6e,00,2e,00,73,00,79,00,73,00,00,00
`,
	SystemRegistry: `Windows Registry Editor Version 5.00

[\ControlSet001\Services\BALLOON]
"DisplayName"=hex(1):40,00,{{ infName|toHex }},2c,00,25,00,42,00,41,00,4c,00,4c,00,4f,00,4f,00,4e,00,2e,00,53,00,56,00,43,00,44,00,45,00,53,00,43,00,25,00,3b,00,56,00,69,00,72,00,74,00,49,00,4f,00,20,00,42,00,61,00,6c,00,6c,00,6f,00,6f,00,6e,00,20,00,53,00,65,00,72,00,76,00,69,00,63,00,65,00,00,00
"ErrorControl"=dword:00000001
"ImagePath"=hex(2):5c,00,53,00,79,00,73,00,74,00,65,00,6d,00,52,00,6f,00,6f,00,74,00,5c,00,53,00,79,00,73,00,74,00,65,00,6d,00,33,00,32,00,5c,00,64,00,72,00,69,00,76,00,65,00,72,00,73,00,5c,00,62,00,61,00,6c,00,6c,00,6f,00,6f,00,6e,00,2e,00,73,00,79,00,73,00,00,00
"Owners"=hex(7):{{ infName|toHex }},00,00,00,00
"Start"=dword:00000003
"Type"=dword:00000001

[\ControlSet001\Services\BALLOON\Parameters]

[\ControlSet001\Services\BALLOON\Parameters\Wdf]
"KmdfLibraryVersion"=hex(1):31,00,2e,00,31,00,35,00,00,00


[\ControlSet001\Services\EventLog\System\BALLOON]
"EventMessageFile"=hex(2):25,00,53,00,79,00,73,00,74,00,65,00,6d,00,52,00,6f,00,6f,00,74,00,25,00,5c,00,53,00,79,00,73,00,74,00,65,00,6d,00,33,00,32,00,5c,00,49,00,6f,00,4c,00,6f,00,67,00,4d,00,73,00,67,00,2e,00,64,00,6c,00,6c,00,3b,00,25,00,53,00,79,00,73,00,74,00,65,00,6d,00,52,00,6f,00,6f,00,74,00,25,00,5c,00,53,00,79,00,73,00,74,00,65,00,6d,00,33,00,32,00,5c,00,64,00,72,00,69,00,76,00,65,00,72,00,73,00,5c,00,62,00,61,00,6c,00,6c,00,6f,00,6f,00,6e,00,2e,00,73,00,79,00,73,00,00,00
"TypesSupported"=dword:00000007


[\DriverDatabase\DeviceIds\PCI\VEN_1AF4&DEV_1002]
"{{ infFile }}"=hex(3):02,ff,00,00

[\DriverDatabase\DeviceIds\PCI\VEN_1AF4&DEV_1002&SUBSYS_00051AF4&REV_00]
"{{ infFile }}"=hex(3):01,ff,00,00

[\DriverDatabase\DeviceIds\PCI\VEN_1AF4&DEV_1045]
"{{ infFile }}"=hex(3):02,ff,00,00

[\DriverDatabase\DeviceIds\PCI\VEN_1AF4&DEV_1045&SUBSYS_11001AF4&REV_01]
"{{ infFile }}"=hex(3):01,ff,00,00

[\DriverDatabase\DeviceIds\{{ classGuid|lower }}]
"{{ infFile }}"=hex(0):

[\DriverDatabase\DriverInfFiles\{{ infFile }}]
@=hex(7):{{ packageName|toHex }},00,00,00,00
"Active"=hex(1):{{ packageName|toHex }},00,00
"Configurations"=hex(7):42,00,41,00,4c,00,4c,00,4f,00,4f,00,4e,00,5f,00,44,00,65,00,76,00,69,00,63,00,65,00,2e,00,4e,00,54,00,00,00,00,00


[\DriverDatabase\DriverPackages\{{ packageName }}]
@=hex(1):{{ infName|toHex }},00,00
"Catalog"=hex(1):42,00,61,00,6c,00,6c,00,6f,00,6f,00,6e,00,2e,00,63,00,61,00,74,00,00,00
"ImportDate"=hex(3):b0,78,d7,bd,ac,e9,d6,01
"InfName"=hex(1):62,00,61,00,6c,00,6c,00,6f,00,6f,00,6e,00,2e,00,69,00,6e,00,66,00,00,00
"OemPath"=hex(1):45,00,3a,00,5c,00,42,00,61,00,6c,00,6c,00,6f,00,6f,00,6e,00,5c,00,77,00,31,00,30,00,5c,00,61,00,6d,00,64,00,36,00,34,00,00,00
"Provider"=hex(1):52,00,65,00,64,00,20,00,48,00,61,00,74,00,2c,00,20,00,49,00,6e,00,63,00,2e,00,00,00
"SignerName"=hex(1):00,00
"SignerScore"=dword:0d000004
"StatusFlags"=dword:00000012
"Version"=hex(3):00,ff,09,00,00,00,00,00,7d,e9,36,4d,25,e3,ce,11,bf,c1,08,00,2b,e1,03,18,00,00,8e,c3,86,b8,d6,01,38,4a,68,00,53,00,64,00,00,00,00,00,00,00,00,00

[\DriverDatabase\DriverPackages\{{ packageName }}\Configurations]

[\DriverDatabase\DriverPackages\{{ packageName }}\Configurations\BALLOON_Device.NT]
"ConfigFlags"=dword:00000000
"ConfigScope"=dword:00000105
"Service"=hex(1):42,00,41,00,4c,00,4c,00,4f,00,4f,00,4e,00,00,00

[\DriverDatabase\DriverPackages\{{ packageName }}\Descriptors]

[\DriverDatabase\DriverPackages\{{ packageName }}\Descriptors\PCI]

[\DriverDatabase\DriverPackages\{{ packageName }}\Descriptors\PCI\VEN_1AF4&DEV_1002]
"Configuration"=hex(1):42,00,41,00,4c,00,4c,00,4f,00,4f,00,4e,00,5f,00,44,00,65,00,76,00,69,00,63,00,65,00,2e,00,4e,00,54,00,00,00
"Description"=hex(1):25,00,62,00,61,00,6c,00,6c,00,6f,00,6f,00,6e,00,2e,00,64,00,65,00,76,00,69,00,63,00,65,00,64,00,65,00,73,00,63,00,25,00,00,00
"Manufacturer"=hex(1):25,00,76,00,65,00,6e,00,64,00,6f,00,72,00,25,00,00,00

[\DriverDatabase\DriverPackages\{{ packageName }}\Descriptors\PCI\VEN_1AF4&DEV_1002&SUBSYS_00051AF4&REV_00]
"Configuration"=hex(1):42,00,41,00,4c,00,4c,00,4f,00,4f,00,4e,00,5f,00,44,00,65,00,76,00,69,00,63,00,65,00,2e,00,4e,00,54,00,00,00
"Description"=hex(1):25,00,62,00,61,00,6c,00,6c,00,6f,00,6f,00,6e,00,2e,00,64,00,65,00,76,00,69,00,63,00,65,00,64,00,65,00,73,00,63,00,25,00,00,00
"Manufacturer"=hex(1):25,00,76,00,65,00,6e,00,64,00,6f,00,72,00,25,00,00,00

[\DriverDatabase\DriverPackages\{{ packageName }}\Descriptors\PCI\VEN_1AF4&DEV_1045]
"Configuration"=hex(1):42,00,41,00,4c,00,4c,00,4f,00,4f,00,4e,00,5f,00,44,00,65,00,76,00,69,00,63,00,65,00,2e,00,4e,00,54,00,00,00
"Description"=hex(1):25,00,62,00,61,00,6c,00,6c,00,6f,00,6f,00,6e,00,2e,00,64,00,65,00,76,00,69,00,63,00,65,00,64,00,65,00,73,00,63,00,25,00,00,00
"Manufacturer"=hex(1):25,00,76,00,65,00,6e,00,64,00,6f,00,72,00,25,00,00,00

[\DriverDatabase\DriverPackages\{{ packageName }}\Descriptors\PCI\VEN_1AF4&DEV_1045&SUBSYS_11001AF4&REV_01]
"Configuration"=hex(1):42,00,41,00,4c,00,4c,00,4f,00,4f,00,4e,00,5f,00,44,00,65,00,76,00,69,00,63,00,65,00,2e,00,4e,00,54,00,00,00
"Description"=hex(1):25,00,62,00,61,00,6c,00,6c,00,6f,00,6f,00,6e,00,2e,00,64,00,65,00,76,00,69,00,63,00,65,00,64,00,65,00,73,00,63,00,25,00,00,00
"Manufacturer"=hex(1):25,00,76,00,65,00,6e,00,64,00,6f,00,72,00,25,00,00,00

[\DriverDatabase\DriverPackages\{{ packageName }}\Strings]
"balloon.devicedesc"=hex(1):56,00,69,00,72,00,74,00,49,00,4f,00,20,00,42,00,61,00,6c,00,6c,00,6f,00,6f,00,6e,00,20,00,44,00,72,00,69,00,76,00,65,00,72,00,00,00
"vendor"=hex(1):52,00,65,00,64,00,20,00,48,00,61,00,74,00,2c,00,20,00,49,00,6e,00,63,00,2e,00,00,00
`,
}
