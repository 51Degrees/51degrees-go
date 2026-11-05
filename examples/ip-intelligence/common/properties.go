package common

// Properties are those the examples show. The free Lite data file has
// RegisteredCountry, RegisteredName and RegisteredOwner, and the Enterprise
// data file has them all.
var Properties = []string{
	"IpRangeStart",
	"IpRangeEnd",
	"AccuracyRadiusMin",
	"AccuracyRadiusMax",
	"RegisteredCountry",
	"RegisteredName",
	"RegisteredOwner",
	"Longitude",
	"Latitude",
	"Areas",
	"Mcc",
}

// NetworkProperties are those the reload and update examples ask for. They
// run on the free Asn data file, which has Asn and AsnName, and the other
// properties need the Enterprise data file.
var NetworkProperties = []string{
	"Asn",
	"AsnName",
	"IpRangeStart",
	"IpRangeEnd",
	"Mcc",
}
