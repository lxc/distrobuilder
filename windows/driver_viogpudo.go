package windows

var driverVioGPUDo = DriverInfo{
	PackageName: "viogpudo.inf_amd64_8224060246e67964",
	DriversRegistry: `[\DriverDatabase\DeviceIds\{{ classGuid|lower }}]
"{{ infFile }}"=hex(0):

[\DriverDatabase\DeviceIds\pci\VEN_1AF4&DEV_1050&SUBSYS_11001AF4&REV_01]
"{{ infFile }}"=hex(3):01,f9,00,00

[\DriverDatabase\DriverInfFiles\{{ infFile }}]
@=hex(7):{{ packageName|toHex }},00,00,00,00
"Active"=hex(1):{{ packageName|toHex }},00,00

[\DriverDatabase\DriverPackages\{{ packageName }}]
@=hex(1):6f,00,65,00,6d,00,30,00,2e,00,69,00,6e,00,66,00,00,00
"Catalog"=hex(1):76,00,69,00,6f,00,67,00,70,00,75,00,64,00,6f,00,2e,00,63,00,61,00,74,00,00,00
"InfName"=hex(1):76,00,69,00,6f,00,67,00,70,00,75,00,64,00,6f,00,2e,00,69,00,6e,00,66,00,00,00
"OemPath"=hex(1):{{ "C:\\Program Files\\Virtio-Win\\"|add:driverName | toHex }},00,00
"Provider"=hex(1):{{"Red Hat, Inc."|toHex}},00,00
"SignerName"=hex(1):{{"Microsoft Windows Hardware Compatibility Publisher"|toHex}},00,00
"SignerScore"=dword:0d000005
"StatusFlags"=dword:00000512
"Version"=hex(3):{{driverVersion}}

[\DriverDatabase\DriverPackages\{{ packageName }}\Properties]

[\DriverDatabase\DriverPackages\{{ packageName }}\Properties\{4da162c1-5eb1-4140-a444-5064c9814e76}]

[\DriverDatabase\DriverPackages\{{ packageName }}\Properties\{4da162c1-5eb1-4140-a444-5064c9814e76}\0009]
@=hex(ffff0012):33,00,30,00,30,00,39,00,37,00,37,00,37,00,30,00,5f,00,31,00,34,00,31,00,35,00,35,00,36,00,33,00,31,00,34,00,35,00,36,00,37,00,30,00,36,00,35,00,39,00,32,00,5f,00,31,00,31,00,35,00,32,00,39,00,32,00,31,00,35,00,30,00,35,00,36,00,39,00,33,00,36,00,38,00,33,00,38,00,34,00,38,00,00,00
`,
}
