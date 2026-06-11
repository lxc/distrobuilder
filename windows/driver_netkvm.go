package windows

var driverNetKVM = DriverInfo{
	PackageName: "netkvm.inf_amd64_805ee20efb26a964",
	DriversRegistry: `[\DriverDatabase\DeviceIds\pci\VEN_1AF4&DEV_1000]
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
"InfName"=hex(1):6e,00,65,00,74,00,6b,00,76,00,6d,00,2e,00,69,00,6e,00,66,00,00,00
"OemPath"=hex(1):{{ "C:\\Program Files\\Virtio-Win\\"|add:driverName | toHex }},00,00
"Provider"=hex(1):{{"Red Hat, Inc."|toHex}},00,00
"SignerName"=hex(1):{{"Microsoft Windows Hardware Compatibility Publisher"|toHex}},00,00
"SignerScore"=dword:0d000004
"StatusFlags"=dword:00000012
"Version"=hex(3):{{driverVersion}}
`,
}
