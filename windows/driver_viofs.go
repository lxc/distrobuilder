package windows

var driverViofs = DriverInfo{
	PackageName: "viofs.inf_amd64_0ea369d39d8c7227",
	SoftwareRegistry: `Windows Registry Editor Version 5.00

[\Microsoft\Windows\CurrentVersion\Setup\PnpLockdownFiles\%SystemRoot%/System32/drivers/viofs.sys]
"Class"=dword:00000004
"Owners"=hex(7):{{ infFile|toHex }},00,00,00,00
"Source"=hex(2):25,00,53,00,79,00,73,00,74,00,65,00,6d,00,52,00,6f,00,6f,00,74,00,25,00,5c,00,53,00,79,00,73,00,74,00,65,00,6d,00,33,00,32,00,5c,00,44,00,72,00,69,00,76,00,65,00,72,00,53,00,74,00,6f,00,72,00,65,00,5c,00,46,00,69,00,6c,00,65,00,52,00,65,00,70,00,6f,00,73,00,69,00,74,00,6f,00,72,00,79,00,5c,00,76,00,69,00,6f,00,66,00,73,00,2e,00,69,00,6e,00,66,00,5f,00,61,00,6d,00,64,00,36,00,34,00,5f,00,30,00,65,00,61,00,33,00,36,00,39,00,64,00,33,00,39,00,64,00,38,00,63,00,37,00,32,00,32,00,37,00,5c,00,76,00,69,00,6f,00,66,00,73,00,2e,00,73,00,79,00,73,00,00,00
`,
	SystemRegistry: `Windows Registry Editor Version 5.00

[\ControlSet001\Services\VirtioFsDrv]
"DisplayName"=hex(1):40,00,{{ infFile|toHex }},2c,00,25,00,56,00,69,00,72,00,74,00,69,00,6f,00,46,00,73,00,2e,00,53,00,65,00,72,00,76,00,69,00,63,00,65,00,25,00,3b,00,56,00,69,00,72,00,74,00,49,00,4f,00,20,00,46,00,53,00,20,00,44,00,72,00,69,00,76,00,65,00,72,00,00,00
"ErrorControl"=dword:00000001
"Group"=hex(1):45,00,78,00,74,00,65,00,6e,00,64,00,65,00,64,00,20,00,42,00,61,00,73,00,65,00,00,00
"ImagePath"=hex(2):5c,00,53,00,79,00,73,00,74,00,65,00,6d,00,52,00,6f,00,6f,00,74,00,5c,00,53,00,79,00,73,00,74,00,65,00,6d,00,33,00,32,00,5c,00,64,00,72,00,69,00,76,00,65,00,72,00,73,00,5c,00,76,00,69,00,6f,00,66,00,73,00,2e,00,73,00,79,00,73,00,00,00
"Owners"=hex(7):{{ infFile|toHex }},00,00,00,00
"Start"=dword:00000003
"Tag"=dword:0000000a
"Type"=dword:00000001

[\ControlSet001\Services\VirtioFsDrv\Parameters]

[\ControlSet001\Services\VirtioFsDrv\Parameters\Wdf]
"KmdfLibraryVersion"=hex(1):31,00,2e,00,31,00,35,00,00,00

[\DriverDatabase\DeviceIds\PCI\VEN_1AF4&DEV_105A]
"{{ infFile }}"=hex(3):02,ff,00,00

[\DriverDatabase\DeviceIds\PCI\VEN_1AF4&DEV_105A&SUBSYS_00041AF4&REV_00]
"{{ infFile }}"=hex(3):01,ff,00,00

[\DriverDatabase\DeviceIds\{{ classGuid|lower }}]
"{{ infFile }}"=hex(0):

[\DriverDatabase\DriverInfFiles\{{ infFile }}]
@=hex(7):{{ packageName|toHex }},00,00,00,00
"Active"=hex(1):{{ packageName|toHex }},00,00
"Configurations"=hex(7):56,00,69,00,72,00,74,00,69,00,6f,00,46,00,73,00,5f,00,44,00,65,00,76,00,69,00,63,00,65,00,2e,00,4e,00,54,00,00,00,00,00

[\DriverDatabase\DriverPackages\{{ packageName }}]
@=hex(1):{{ infFile|toHex }},00,00
"Catalog"=hex(1):76,00,69,00,6f,00,66,00,73,00,2e,00,63,00,61,00,74,00,00,00
"ImportDate"=hex(3):60,e0,9c,62,9e,ee,d6,01
"InfName"=hex(1):76,00,69,00,6f,00,66,00,73,00,2e,00,69,00,6e,00,66,00,00,00
"OemPath"=hex(1):45,00,3a,00,5c,00,76,00,69,00,6f,00,66,00,73,00,5c,00,77,00,31,00,30,00,5c,00,61,00,6d,00,64,00,36,00,34,00,00,00
"Provider"=hex(1):52,00,65,00,64,00,20,00,48,00,61,00,74,00,2c,00,20,00,49,00,6e,00,63,00,2e,00,00,00
"SignerName"=hex(1):00,00
"SignerScore"=dword:0d000004
"StatusFlags"=dword:00000012
"Version"=hex(3):00,ff,09,00,00,00,00,00,7d,e9,36,4d,25,e3,ce,11,bf,c1,08,00,2b,e1,03,18,00,00,8e,c3,86,b8,d6,01,38,4a,68,00,53,00,64,00,00,00,00,00,00,00,00,00

[\DriverDatabase\DriverPackages\{{ packageName }}\Configurations]

[\DriverDatabase\DriverPackages\{{ packageName }}\Configurations\VirtioFs_Device.NT]
"ConfigFlags"=dword:00000000
"ConfigScope"=dword:00000107
"Service"=hex(1):56,00,69,00,72,00,74,00,69,00,6f,00,46,00,73,00,44,00,72,00,76,00,00,00

[\DriverDatabase\DriverPackages\{{ packageName }}\Configurations\VirtioFs_Device.NT\Device]

[\DriverDatabase\DriverPackages\{{ packageName }}\Configurations\VirtioFs_Device.NT\Device\Interrupt Management]

[\DriverDatabase\DriverPackages\{{ packageName }}\Configurations\VirtioFs_Device.NT\Device\Interrupt Management\MessageSignaledInterruptProperties]
"MSISupported"=dword:00000001
"MessageNumberLimit"=dword:00000001

[\DriverDatabase\DriverPackages\{{ packageName }}\Descriptors]

[\DriverDatabase\DriverPackages\{{ packageName }}\Descriptors\PCI]

[\DriverDatabase\DriverPackages\{{ packageName }}\Descriptors\PCI\VEN_1AF4&DEV_105A]
"Configuration"=hex(1):56,00,69,00,72,00,74,00,69,00,6f,00,46,00,73,00,5f,00,44,00,65,00,76,00,69,00,63,00,65,00,2e,00,4e,00,54,00,00,00
"Description"=hex(1):25,00,76,00,69,00,72,00,74,00,69,00,6f,00,66,00,73,00,2e,00,64,00,65,00,76,00,69,00,63,00,65,00,64,00,65,00,73,00,63,00,25,00,00,00
"Manufacturer"=hex(1):25,00,76,00,65,00,6e,00,64,00,6f,00,72,00,25,00,00,00

[\DriverDatabase\DriverPackages\{{ packageName }}\Descriptors\PCI\VEN_1AF4&DEV_105A&SUBSYS_00041AF4&REV_00]
"Configuration"=hex(1):56,00,69,00,72,00,74,00,69,00,6f,00,46,00,73,00,5f,00,44,00,65,00,76,00,69,00,63,00,65,00,2e,00,4e,00,54,00,00,00
"Description"=hex(1):25,00,76,00,69,00,72,00,74,00,69,00,6f,00,66,00,73,00,2e,00,64,00,65,00,76,00,69,00,63,00,65,00,64,00,65,00,73,00,63,00,25,00,00,00
"Manufacturer"=hex(1):25,00,76,00,65,00,6e,00,64,00,6f,00,72,00,25,00,00,00

[\DriverDatabase\DriverPackages\{{ packageName }}\Strings]
"vendor"=hex(1):52,00,65,00,64,00,20,00,48,00,61,00,74,00,2c,00,20,00,49,00,6e,00,63,00,2e,00,00,00
"virtiofs.devicedesc"=hex(1):56,00,69,00,72,00,74,00,49,00,4f,00,20,00,46,00,53,00,20,00,44,00,65,00,76,00,69,00,63,00,65,00,00,00
`,
}
