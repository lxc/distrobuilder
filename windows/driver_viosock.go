package windows

var driverViosock = DriverInfo{
	PackageName: "viosock.inf_amd64_ab8c47eeb7b8c3b3",
	SoftwareRegistry: `
[\Microsoft\Windows\CurrentVersion\Setup\PnpLockdownFiles\%SystemRoot%/SysWOW64/viosocklib.dll]
"Class"=dword:00000004
"Owners"=hex(7):{{ infFile|toHex }},00,00,00,00
"Source"=hex(2):{{"%SystemRoot%\\System32\\DriverStore\\FileRepository\\"|add:packageName|add:"\\viosocklib_x86.dll"|toHex}},00,00

[\Microsoft\Windows\CurrentVersion\Setup\PnpLockdownFiles\%SystemRoot%/System32/viosocklib.dll]
"Class"=dword:00000004
"Owners"=hex(7):{{ infFile|toHex }},00,00,00,00
"Source"=hex(2):{{"%SystemRoot%\\System32\\DriverStore\\FileRepository\\"|add:packageName|add:"\\viosocklib_x64.dll"|toHex}},00,00

[\Microsoft\Windows\CurrentVersion\Setup\PnpLockdownFiles\%SystemRoot%/System32/viosockwspsvc.exe]
"Class"=dword:00000004
"Owners"=hex(7):{{ infFile|toHex }},00,00,00,00
"Source"=hex(2):{{"%SystemRoot%\\System32\\DriverStore\\FileRepository\\"|add:packageName|add:"\\viosockwspsvc.exe"|toHex}},00,00
`,
	SystemRegistry: `
[\ControlSet001\Services\VirtioSocket]
"DisplayName"=hex(1):40,00,{{ infFile|toHex }},{{ ",%VirtioSocket.ServiceDesc%;VirtIO Socket Service"|toHex }},00,00
"ErrorControl"=dword:00000001
"ImagePath"=hex(2):{{ "\\SystemRoot\\System32\\DriverStore\\FileRepository\\"|add:packageName|add:"\\viosock.sys"|toHex }},00,00
"Owners"=hex(7):{{ infFile|toHex }},00,00,00,00
"Start"=dword:00000003
"Type"=dword:00000001

[\ControlSet001\Services\VirtioSocket\Parameters]

[\ControlSet001\Services\VirtioSocket\Parameters\Wdf]
"KmdfLibraryVersion"=hex(1):{{ "1.15"|toHex }},00,00

[\ControlSet001\Services\VirtioSocketWSP]
"DisplayName"=hex(1):40,00,{{ infFile|toHex }},{{ ",%VirtioSocketWSP.ServiceDesc%;VirtIO Socket WSP Service"|toHex }},00,00
"ErrorControl"=dword:00000001
"ImagePath"=hex(2):{{ "%SystemRoot%\\System32\\viosockwspsvc.exe"|toHex }},00,00
"ObjectName"=hex(1):{{"LocalSystem"|toHex}},00,00
"Owners"=hex(7):{{ infFile|toHex }},00,00,00,00
"Start"=dword:00000002
"Type"=dword:00000010

[\DriverDatabase\DeviceIds\{{ classGuid|lower }}]
"{{ infFile }}"=hex(0):

[\DriverDatabase\DeviceIds\PCI\VEN_1AF4&DEV_1053&SUBSYS_11001AF4&REV_01]
"{{ infFile }}"=hex(3):01,ff,00,00

[\DriverDatabase\DeviceIds\PCI\VEN_1AF4&DEV_1012]
"{{ infFile }}"=hex(3):02,ff,00,00

[\DriverDatabase\DeviceIds\PCI\VEN_1AF4&DEV_1012&SUBSYS_00131AF4&REV_00]
"{{ infFile }}"=hex(3):01,ff,00,00

[\DriverDatabase\DeviceIds\PCI\VEN_1AF4&DEV_1053]
"{{ infFile }}"=hex(3):02,ff,00,00

[\DriverDatabase\DriverInfFiles\{{ infFile }}]
@=hex(7):{{ packageName|toHex }},00,00,00,00
"Active"=hex(1):{{ packageName|toHex }},00,00
"Configurations"=hex(7):{{"VirtioSocket_Device.NT"|toHex }},00,00,00,00

[\DriverDatabase\DriverPackages\{{ packageName }}]
@=hex(1):{{ infFile|toHex }},00,00
"Catalog"=hex(1):{{"viosock.cat"|toHex}},00,00
"CatalogAttributes"=dword:80000003
"FileSize"=hex(b):ed,fa,07,00,00,00,00,00
"ImportDate"=hex(3):20,ab,d7,47,8c,2d,dc,01
"InfName"=hex(1):{{"viosock.inf"|toHex}},00,00
"LockLevel"=dword:00000002
"OemPath"=hex(1):{{"E:\\viosock\\2k25\\amd64"|toHex}},00,00
"OsVersionFloor"=dword:0a003fab
"Provider"=hex(1):{{"Red Hat, Inc."|toHex}},00,00
"SignerName"=hex(1):{{"Microsoft Windows Hardware Compatibility Publisher"|toHex}},00,00
"SignerScore"=dword:0d000005
"StatusFlags"=dword:00000033
"Version"=hex(3):00,ff,09,00,00,00,00,00,7d,e9,36,4d,25,e3,ce,11,bf,c1,08,00,2b,e1,03,18,00,00,cd,69,64,f0,db,01,54,6f,68,00,65,00,64,00,00,00,00,00,00,00,00,00

[\DriverDatabase\DriverPackages\{{ packageName }}\Configurations]

[\DriverDatabase\DriverPackages\{{ packageName }}\Configurations\VirtioSocket_Device.NT]
"ConfigFlags"=dword:00000000
"ConfigScope"=dword:00000f7f
"Service"=hex(1):{{"VirtioSocket"|toHex}},00,00
"Win32Services"=hex(7):{{"VirtioSocketWSP"|toHex}},00,00,00,00

[\DriverDatabase\DriverPackages\{{ packageName }}\Descriptors]

[\DriverDatabase\DriverPackages\{{ packageName }}\Descriptors\PCI]

[\DriverDatabase\DriverPackages\{{ packageName }}\Descriptors\PCI\VEN_1AF4&DEV_1053&SUBSYS_11001AF4&REV_01]
"Configuration"=hex(1):{{"VirtioSocket_Device.NT"|toHex}},00,00
"Description"=hex(1):{{"%virtiosocket.devicedesc%"|toHex}},00,00
"Manufacturer"=hex(1):{{"%vendor%"|toHex}},00,00

[\DriverDatabase\DriverPackages\{{ packageName }}\Descriptors\PCI\VEN_1AF4&DEV_1053]
"Configuration"=hex(1):{{"VirtioSocket_Device.NT"|toHex}},00,00
"Description"=hex(1):{{"%virtiosocket.devicedesc%"|toHex}},00,00
"Manufacturer"=hex(1):{{"%vendor%"|toHex}},00,00

[\DriverDatabase\DriverPackages\{{ packageName }}\Strings]
"vendor"=hex(1):{{"Red Hat, Inc."|toHex}},00,00
"virtiosocket.devicedesc"=hex(1):{{"VirtIO Socket Driver"|toHex}},00,00
`,
}
