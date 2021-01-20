package windows

// DriverInfo contains driver specific information.
type DriverInfo struct {
	PackageName      string
	SoftwareRegistry string
	SystemRegistry   string
	DriversRegistry  string
}

// Drivers contains all supported drivers.
var Drivers = map[string]DriverInfo{
	"Balloon":   driverBalloon,
	"NetKVM":    driverNetKVM,
	"vioinput":  driverVioinput,
	"viorng":    driverViorng,
	"vioscsi":   driverVioscsi,
	"vioserial": driverVioserial,
	"viofs":     driverViofs,
}
