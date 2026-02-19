package windows

// DriverInfo contains driver specific information.
type DriverInfo struct {
	PackageName      string
	SoftwareRegistry string
	SystemRegistry   string
	DriversRegistry  string
	SystemCatalog    []CatalogEntry
}

// CatalogEntry represents entries in a registry catalog that are indexed.
type CatalogEntry struct {
	// Key in the pongo2 context for this entry.
	CtxKey string

	// Some unique identifier to search for so we don't duplicate entries.
	ID string

	// Path to the catalog root.
	Path string

	// Registry content to add.
	Content string
}

// CatalogIndex maps to the index record for a registry catalog.
type CatalogIndex struct {
	NextEntryID     int
	NumEntries      int
	NumEntries64    int
	SerialAccessNum int
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
	"viogpudo":  driverVioGPUDo,
	"viostor":   driverViostor,
	"viosock":   driverViosock,
}
