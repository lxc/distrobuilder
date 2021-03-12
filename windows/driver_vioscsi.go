package windows

var driverVioscsi = DriverInfo{
	PackageName: "vioscsi.inf_amd64_78d23e29bdcf3e06",
	SoftwareRegistry: `[\Microsoft\Windows\CurrentVersion\Setup\PnpLockdownFiles\%SystemRoot%/System32/drivers/vioscsi.sys]
"Class"=dword:00000005
"Owners"=hex(7):{{ infFile|toHex }},00,00,00,00
"Source"=hex(2):25,00,53,00,79,00,73,00,74,00,65,00,6d,00,52,00,6f,00,6f,00,74,00,25,00,5c,00,53,00,79,00,73,00,74,00,65,00,6d,00,33,00,32,00,5c,00,44,00,72,00,69,00,76,00,65,00,72,00,53,00,74,00,6f,00,72,00,65,00,5c,00,46,00,69,00,6c,00,65,00,52,00,65,00,70,00,6f,00,73,00,69,00,74,00,6f,00,72,00,79,00,5c,00,76,00,69,00,6f,00,73,00,63,00,73,00,69,00,2e,00,69,00,6e,00,66,00,5f,00,61,00,6d,00,64,00,36,00,34,00,5f,00,37,00,38,00,64,00,32,00,33,00,65,00,32,00,39,00,62,00,64,00,63,00,66,00,33,00,65,00,30,00,36,00,5c,00,76,00,69,00,6f,00,73,00,63,00,73,00,69,00,2e,00,73,00,79,00,73,00,00,00
`,
	SystemRegistry: `[\DriverDatabase]
"OemInfMap"=hex(3):e0

[\DriverDatabase\DeviceIds\{{ classGuid|lower }}]
"{{ infFile }}"=hex(0):

[\ControlSet001\Services\EventLog\System\{{ driverName }}]
"EventMessageFile"=hex(2):25,00,53,00,79,00,73,00,74,00,65,00,6d,00,52,00,6f,00,6f,00,74,00,25,00,5c,00,53,00,79,00,73,00,74,00,65,00,6d,00,33,00,32,00,5c,00,49,00,6f,00,4c,00,6f,00,67,00,4d,00,73,00,67,00,2e,00,64,00,6c,00,6c,00,00,00
"TypesSupported"=dword:00000007

[\ControlSet001\Services\{{ driverName }}]
"DisplayName"=hex(1):40,00,{{ infFile|toHex }},2c,00,25,00,56,00,69,00,72,00,74,00,69,00,6f,00,53,00,63,00,73,00,69,00,2e,00,53,00,56,00,43,00,44,00,45,00,53,00,43,00,25,00,3b,00,52,00,65,00,64,00,20,00,48,00,61,00,74,00,20,00,56,00,69,00,72,00,74,00,49,00,4f,00,20,00,53,00,43,00,53,00,49,00,20,00,70,00,61,00,73,00,73,00,2d,00,74,00,68,00,72,00,6f,00,75,00,67,00,68,00,20,00,53,00,65,00,72,00,76,00,69,00,63,00,65,00,00,00
"ErrorControl"=dword:00000001
"Group"=hex(1):53,00,43,00,53,00,49,00,20,00,6d,00,69,00,6e,00,69,00,70,00,6f,00,72,00,74,00,00,00
"ImagePath"=hex(2):53,00,79,00,73,00,74,00,65,00,6d,00,33,00,32,00,5c,00,64,00,72,00,69,00,76,00,65,00,72,00,73,00,5c,00,{{ driverName|toHex }},2e,00,73,00,79,00,73,00,00,00
"Owners"=hex(7):{{ infFile|toHex }},00,00,00,00
"Start"=dword:00000000
"Tag"=dword:00000021
"Type"=dword:00000001

[\ControlSet001\Services\{{ driverName }}\Parameters]
"BusType"=dword:0000000a

[\ControlSet001\Services\{{ driverName }}\Parameters\PnpInterface]
"5"=dword:00000001

[\DriverDatabase\DeviceIds\PCI\VEN_1AF4&DEV_1004]
"{{ infFile }}"=hex(3):02,ff,00,00

[\DriverDatabase\DeviceIds\PCI\VEN_1AF4&DEV_1004&SUBSYS_00081AF4&REV_00]
"{{ infFile }}"=hex(3):01,ff,00,00

[\DriverDatabase\DeviceIds\PCI\VEN_1AF4&DEV_1048]
"{{ infFile }}"=hex(3):02,ff,00,00

[\DriverDatabase\DeviceIds\PCI\VEN_1AF4&DEV_1048&SUBSYS_11001AF4&REV_01]
"{{ infFile }}"=hex(3):01,ff,00,00

[\DriverDatabase\DriverInfFiles\{{ infFile }}]
@=hex(7):{{ packageName|toHex }},00,00,00,00
"Active"=hex(1):{{ packageName|toHex }},00,00
"Configurations"=hex(7):73,00,63,00,73,00,69,00,5f,00,69,00,6e,00,73,00,74,00,00,00,00,00

[\DriverDatabase\DriverPackages\{{ packageName }}]
@=hex(1):{{ infFile|toHex }},00,00
"Catalog"=hex(1):{{ driverName|toHex }},2e,00,63,00,61,00,74,00,00,00
"ImportDate"=hex(3):30,f6,fd,27,c8,c7,d6,01
"InfName"=hex(1):{{ driverName|toHex }},2e,00,69,00,6e,00,66,00,00,00
"OemPath"=hex(1):43,00,3a,00,5c,00,55,00,73,00,65,00,72,00,73,00,5c,00,54,00,68,00,6f,00,6d,00,61,00,73,00,5c,00,44,00,6f,00,77,00,6e,00,6c,00,6f,00,61,00,64,00,73,00,5c,00,64,00,72,00,69,00,76,00,65,00,72,00,73,00,00,00
"Provider"=hex(1):52,00,65,00,64,00,20,00,48,00,61,00,74,00,2c,00,20,00,49,00,6e,00,63,00,2e,00,00,00
"SignerName"=hex(1):00,00
"SignerScore"=dword:0d000004
"StatusFlags"=dword:00000012
"Version"=hex(3):00,ff,09,00,00,00,00,00,7b,e9,36,4d,25,e3,ce,11,bf,c1,08,00,2b,e1,03,18,00,00,8e,c3,86,b8,d6,01,38,4a,68,00,53,00,64,00,00,00,00,00,00,00,00,00

[\DriverDatabase\DriverPackages\{{ packageName }}\Configurations]

[\DriverDatabase\DriverPackages\{{ packageName }}\Configurations\scsi_inst]
"ConfigFlags"=dword:00000000
"ConfigScope"=dword:00000007
"Service"=hex(1):{{ driverName|toHex }},00,00

[\DriverDatabase\DriverPackages\{{ packageName }}\Configurations\scsi_inst\Device]

[\DriverDatabase\DriverPackages\{{ packageName }}\Configurations\scsi_inst\Device\Interrupt Management]

[\DriverDatabase\DriverPackages\{{ packageName }}\Configurations\scsi_inst\Device\Interrupt Management\Affinity Policy]
"DevicePolicy"=dword:00000005
"DevicePriority"=dword:00000003

[\DriverDatabase\DriverPackages\{{ packageName }}\Configurations\scsi_inst\Device\Interrupt Management\MessageSignaledInterruptProperties]
"MSISupported"=dword:00000001
"MessageNumberLimit"=dword:00000100

[\DriverDatabase\DriverPackages\{{ packageName }}\Configurations\scsi_inst\Services]

[\DriverDatabase\DriverPackages\{{ packageName }}\Configurations\scsi_inst\Services\{{ driverName }}]

[\DriverDatabase\DriverPackages\{{ packageName }}\Configurations\scsi_inst\Services\{{ driverName }}\Parameters]
"BusType"=dword:0000000a

[\DriverDatabase\DriverPackages\{{ packageName }}\Configurations\scsi_inst\Services\{{ driverName }}\Parameters\PnpInterface]
"5"=dword:00000001

[\DriverDatabase\DriverPackages\{{ packageName }}\Descriptors]

[\DriverDatabase\DriverPackages\{{ packageName }}\Descriptors\PCI]

[\DriverDatabase\DriverPackages\{{ packageName }}\Descriptors\PCI\VEN_1AF4&DEV_1004]
"Configuration"=hex(1):73,00,63,00,73,00,69,00,5f,00,69,00,6e,00,73,00,74,00,00,00
"Description"=hex(1):25,00,76,00,69,00,72,00,74,00,69,00,6f,00,73,00,63,00,73,00,69,00,2e,00,64,00,65,00,76,00,69,00,63,00,65,00,64,00,65,00,73,00,63,00,25,00,00,00
"Manufacturer"=hex(1):25,00,76,00,65,00,6e,00,64,00,6f,00,72,00,25,00,00,00

[\DriverDatabase\DriverPackages\{{ packageName }}\Descriptors\PCI\VEN_1AF4&DEV_1004&SUBSYS_00081AF4&REV_00]
"Configuration"=hex(1):73,00,63,00,73,00,69,00,5f,00,69,00,6e,00,73,00,74,00,00,00
"Description"=hex(1):25,00,76,00,69,00,72,00,74,00,69,00,6f,00,73,00,63,00,73,00,69,00,2e,00,64,00,65,00,76,00,69,00,63,00,65,00,64,00,65,00,73,00,63,00,25,00,00,00
"Manufacturer"=hex(1):25,00,76,00,65,00,6e,00,64,00,6f,00,72,00,25,00,00,00

[\DriverDatabase\DriverPackages\{{ packageName }}\Descriptors\PCI\VEN_1AF4&DEV_1048]
"Configuration"=hex(1):73,00,63,00,73,00,69,00,5f,00,69,00,6e,00,73,00,74,00,00,00
"Description"=hex(1):25,00,76,00,69,00,72,00,74,00,69,00,6f,00,73,00,63,00,73,00,69,00,2e,00,64,00,65,00,76,00,69,00,63,00,65,00,64,00,65,00,73,00,63,00,25,00,00,00
"Manufacturer"=hex(1):25,00,76,00,65,00,6e,00,64,00,6f,00,72,00,25,00,00,00

[\DriverDatabase\DriverPackages\{{ packageName }}\Descriptors\PCI\VEN_1AF4&DEV_1048&SUBSYS_11001AF4&REV_01]
"Configuration"=hex(1):73,00,63,00,73,00,69,00,5f,00,69,00,6e,00,73,00,74,00,00,00
"Description"=hex(1):25,00,76,00,69,00,72,00,74,00,69,00,6f,00,73,00,63,00,73,00,69,00,2e,00,64,00,65,00,76,00,69,00,63,00,65,00,64,00,65,00,73,00,63,00,25,00,00,00
"Manufacturer"=hex(1):25,00,76,00,65,00,6e,00,64,00,6f,00,72,00,25,00,00,00

[\DriverDatabase\DriverPackages\{{ packageName }}\Strings]
"vendor"=hex(1):52,00,65,00,64,00,20,00,48,00,61,00,74,00,2c,00,20,00,49,00,6e,00,63,00,2e,00,00,00
"virtioscsi.devicedesc"=hex(1):52,00,65,00,64,00,20,00,48,00,61,00,74,00,20,00,56,00,69,00,72,00,74,00,49,00,4f,00,20,00,53,00,43,00,53,00,49,00,20,00,70,00,61,00,73,00,73,00,2d,00,74,00,68,00,72,00,6f,00,75,00,67,00,68,00,20,00,63,00,6f,00,6e,00,74,00,72,00,6f,00,6c,00,6c,00,65,00,72,00,00,00
`,
}
