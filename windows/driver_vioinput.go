package windows

var driverVioinput = DriverInfo{
	PackageName: "vioinput.inf_amd64_e4dfa6cdfd16da9a",
	SoftwareRegistry: `Windows Registry Editor Version 5.00

[\Microsoft\Windows\CurrentVersion\Setup\PnpLockdownFiles\%SystemRoot%/System32/drivers/viohidkmdf.sys]
"Class"=dword:00000004
"Owners"=hex(7):{{ infFile|toHex }},00,00,00,00
"Source"=hex(2):25,00,53,00,79,00,73,00,74,00,65,00,6d,00,52,00,6f,00,6f,00,74,00,25,00,5c,00,53,00,79,00,73,00,74,00,65,00,6d,00,33,00,32,00,5c,00,44,00,72,00,69,00,76,00,65,00,72,00,53,00,74,00,6f,00,72,00,65,00,5c,00,46,00,69,00,6c,00,65,00,52,00,65,00,70,00,6f,00,73,00,69,00,74,00,6f,00,72,00,79,00,5c,00,76,00,69,00,6f,00,69,00,6e,00,70,00,75,00,74,00,2e,00,69,00,6e,00,66,00,5f,00,61,00,6d,00,64,00,36,00,34,00,5f,00,65,00,34,00,64,00,66,00,61,00,36,00,63,00,64,00,66,00,64,00,31,00,36,00,64,00,61,00,39,00,61,00,5c,00,76,00,69,00,6f,00,68,00,69,00,64,00,6b,00,6d,00,64,00,66,00,2e,00,73,00,79,00,73,00,00,00

[\Microsoft\Windows\CurrentVersion\Setup\PnpLockdownFiles\%SystemRoot%/System32/drivers/vioinput.sys]
"Class"=dword:00000004
"Owners"=hex(7):{{ infFile|toHex }},00,00,00,00
"Source"=hex(2):25,00,53,00,79,00,73,00,74,00,65,00,6d,00,52,00,6f,00,6f,00,74,00,25,00,5c,00,53,00,79,00,73,00,74,00,65,00,6d,00,33,00,32,00,5c,00,44,00,72,00,69,00,76,00,65,00,72,00,53,00,74,00,6f,00,72,00,65,00,5c,00,46,00,69,00,6c,00,65,00,52,00,65,00,70,00,6f,00,73,00,69,00,74,00,6f,00,72,00,79,00,5c,00,76,00,69,00,6f,00,69,00,6e,00,70,00,75,00,74,00,2e,00,69,00,6e,00,66,00,5f,00,61,00,6d,00,64,00,36,00,34,00,5f,00,65,00,34,00,64,00,66,00,61,00,36,00,63,00,64,00,66,00,64,00,31,00,36,00,64,00,61,00,39,00,61,00,5c,00,76,00,69,00,6f,00,69,00,6e,00,70,00,75,00,74,00,2e,00,73,00,79,00,73,00,00,00
	`,
	SystemRegistry: `Windows Registry Editor Version 5.00

[\DriverDatabase\DeviceIds\{{ classGuid|lower }}]
"{{ infFile }}"=hex(0):

[\ControlSet001\Services\VirtioInput]
"DisplayName"=hex(1):40,00,{{ infFile|toHex }},2c,00,25,00,56,00,69,00,72,00,74,00,69,00,6f,00,49,00,6e,00,70,00,75,00,74,00,2e,00,53,00,65,00,72,00,76,00,69,00,63,00,65,00,44,00,65,00,73,00,63,00,25,00,3b,00,56,00,69,00,72,00,74,00,49,00,4f,00,20,00,49,00,6e,00,70,00,75,00,74,00,20,00,53,00,65,00,72,00,76,00,69,00,63,00,65,00,00,00
"ErrorControl"=dword:00000001
"ImagePath"=hex(2):5c,00,53,00,79,00,73,00,74,00,65,00,6d,00,52,00,6f,00,6f,00,74,00,5c,00,53,00,79,00,73,00,74,00,65,00,6d,00,33,00,32,00,5c,00,64,00,72,00,69,00,76,00,65,00,72,00,73,00,5c,00,{{ driverName|toHex }},2e,00,73,00,79,00,73,00,00,00
"Owners"=hex(7):{{ infFile|toHex }},00,00,00,00
"Start"=dword:00000003
"Type"=dword:00000001

[\ControlSet001\Services\VirtioInput\Parameters]

[\ControlSet001\Services\VirtioInput\Parameters\Wdf]
"KmdfLibraryVersion"=hex(1):31,00,2e,00,31,00,35,00,00,00

[\ControlSet001\Services\viohidkmdf]
"ErrorControl"=dword:00000001
"ImagePath"=hex(2):5c,00,53,00,79,00,73,00,74,00,65,00,6d,00,52,00,6f,00,6f,00,74,00,5c,00,53,00,79,00,73,00,74,00,65,00,6d,00,33,00,32,00,5c,00,64,00,72,00,69,00,76,00,65,00,72,00,73,00,5c,00,76,00,69,00,6f,00,68,00,69,00,64,00,6b,00,6d,00,64,00,66,00,2e,00,73,00,79,00,73,00,00,00
"Owners"=hex(7):{{ infFile|toHex }},00,00,00,00
"Start"=dword:00000003
"Type"=dword:00000001

[\DriverDatabase\DeviceIds\PCI\VEN_1AF4&DEV_1052]
"{{ infFile }}"=hex(3):02,ff,00,00

[\DriverDatabase\DeviceIds\PCI\VEN_1AF4&DEV_1052&SUBSYS_11001AF4&REV_01]
"{{ infFile }}"=hex(3):01,ff,00,00

[\DriverDatabase\DeviceIds\VIOINPUT]

[\DriverDatabase\DeviceIds\VIOINPUT\REV_01]
"{{ infFile }}"=hex(3):01,ff,00,00

[\DriverDatabase\DriverInfFiles\{{ infFile }}]
@=hex(7):{{ packageName|toHex }},00,00,00,00
"Active"=hex(1):{{ packageName|toHex }},00,00
"Configurations"=hex(7):56,00,69,00,72,00,74,00,69,00,6f,00,49,00,6e,00,70,00,75,00,74,00,5f,00,44,00,65,00,76,00,69,00,63,00,65,00,2e,00,4e,00,54,00,00,00,56,00,69,00,72,00,74,00,69,00,6f,00,49,00,6e,00,70,00,75,00,74,00,5f,00,43,00,68,00,69,00,6c,00,64,00,2e,00,4e,00,54,00,00,00,00,00

[\DriverDatabase\DriverPackages\{{ packageName }}]
@=hex(1):{{ infFile|toHex }},00,00
"Catalog"=hex(1):{{ driverName|toHex }},2e,00,63,00,61,00,74,00,00,00
"ImportDate"=hex(3):b0,1d,ca,bf,fb,e7,d6,01
"InfName"=hex(1):{{ driverName|toHex }},2e,00,69,00,6e,00,66,00,00,00
"OemPath"=hex(1):45,00,3a,00,5c,00,{{ driverName|toHex }},5c,00,77,00,31,00,30,00,5c,00,61,00,6d,00,64,00,36,00,34,00,00,00
"Provider"=hex(1):52,00,65,00,64,00,20,00,48,00,61,00,74,00,2c,00,20,00,49,00,6e,00,63,00,2e,00,00,00
"SignerName"=hex(1):00,00
"SignerScore"=dword:0d000004
"StatusFlags"=dword:00000012
"Version"=hex(3):00,ff,09,00,00,00,00,00,a0,17,5a,74,d3,74,d0,11,b6,fe,00,a0,c9,0f,57,da,00,00,8e,c3,86,b8,d6,01,38,4a,68,00,53,00,64,00,00,00,00,00,00,00,00,00

[\DriverDatabase\DriverPackages\{{ packageName }}\Configurations]

[\DriverDatabase\DriverPackages\{{ packageName }}\Configurations\VirtioInput_Child.NT]
"ConfigFlags"=dword:00000000
"ConfigScope"=dword:00000005
"Service"=hex(1):76,00,69,00,6f,00,68,00,69,00,64,00,6b,00,6d,00,64,00,66,00,00,00

[\DriverDatabase\DriverPackages\{{ packageName }}\Configurations\VirtioInput_Device.NT]
"ConfigFlags"=dword:00000000
"ConfigScope"=dword:00000107
"Service"=hex(1):56,00,69,00,72,00,74,00,69,00,6f,00,49,00,6e,00,70,00,75,00,74,00,00,00

[\DriverDatabase\DriverPackages\{{ packageName }}\Configurations\VirtioInput_Device.NT\Device]

[\DriverDatabase\DriverPackages\{{ packageName }}\Configurations\VirtioInput_Device.NT\Device\Interrupt Management]

[\DriverDatabase\DriverPackages\{{ packageName }}\Configurations\VirtioInput_Device.NT\Device\Interrupt Management\MessageSignaledInterruptProperties]
"MSISupported"=dword:00000001
"MessageNumberLimit"=dword:00000002

[\DriverDatabase\DriverPackages\{{ packageName }}\Descriptors]

[\DriverDatabase\DriverPackages\{{ packageName }}\Descriptors\PCI]

[\DriverDatabase\DriverPackages\{{ packageName }}\Descriptors\PCI\VEN_1AF4&DEV_1052]
"Configuration"=hex(1):56,00,69,00,72,00,74,00,69,00,6f,00,49,00,6e,00,70,00,75,00,74,00,5f,00,44,00,65,00,76,00,69,00,63,00,65,00,2e,00,4e,00,54,00,00,00
"Description"=hex(1):25,00,76,00,69,00,72,00,74,00,69,00,6f,00,69,00,6e,00,70,00,75,00,74,00,2e,00,64,00,65,00,76,00,69,00,63,00,65,00,64,00,65,00,73,00,63,00,25,00,00,00
"Manufacturer"=hex(1):25,00,76,00,65,00,6e,00,64,00,6f,00,72,00,25,00,00,00

[\DriverDatabase\DriverPackages\{{ packageName }}\Descriptors\PCI\VEN_1AF4&DEV_1052&SUBSYS_11001AF4&REV_01]
"Configuration"=hex(1):56,00,69,00,72,00,74,00,69,00,6f,00,49,00,6e,00,70,00,75,00,74,00,5f,00,44,00,65,00,76,00,69,00,63,00,65,00,2e,00,4e,00,54,00,00,00
"Description"=hex(1):25,00,76,00,69,00,72,00,74,00,69,00,6f,00,69,00,6e,00,70,00,75,00,74,00,2e,00,64,00,65,00,76,00,69,00,63,00,65,00,64,00,65,00,73,00,63,00,25,00,00,00
"Manufacturer"=hex(1):25,00,76,00,65,00,6e,00,64,00,6f,00,72,00,25,00,00,00

[\DriverDatabase\DriverPackages\{{ packageName }}\Descriptors\VIOINPUT]

[\DriverDatabase\DriverPackages\{{ packageName }}\Descriptors\VIOINPUT\REV_01]
"Configuration"=hex(1):56,00,69,00,72,00,74,00,69,00,6f,00,49,00,6e,00,70,00,75,00,74,00,5f,00,43,00,68,00,69,00,6c,00,64,00,2e,00,4e,00,54,00,00,00
"Description"=hex(1):25,00,76,00,69,00,72,00,74,00,69,00,6f,00,69,00,6e,00,70,00,75,00,74,00,2e,00,63,00,68,00,69,00,6c,00,64,00,64,00,65,00,73,00,63,00,25,00,00,00
"Manufacturer"=hex(1):25,00,76,00,65,00,6e,00,64,00,6f,00,72,00,25,00,00,00

[\DriverDatabase\DriverPackages\{{ packageName }}\Strings]
"vendor"=hex(1):52,00,65,00,64,00,20,00,48,00,61,00,74,00,2c,00,20,00,49,00,6e,00,63,00,2e,00,00,00
"virtioinput.childdesc"=hex(1):56,00,69,00,72,00,74,00,49,00,4f,00,20,00,49,00,6e,00,70,00,75,00,74,00,20,00,44,00,72,00,69,00,76,00,65,00,72,00,20,00,48,00,65,00,6c,00,70,00,65,00,72,00,00,00
"virtioinput.devicedesc"=hex(1):56,00,69,00,72,00,74,00,49,00,4f,00,20,00,49,00,6e,00,70,00,75,00,74,00,20,00,44,00,72,00,69,00,76,00,65,00,72,00,00,00
`,
}
