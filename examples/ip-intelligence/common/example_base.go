package common

import (
	"os"
)

// LiteDataFile is the free data file the examples use when DATA_FILE names no
// other. The data repository links it, so an example downloads it the first
// time it runs.
const LiteDataFile = "51Degrees-LiteV41.ipi"

// AsnDataFile is the free data file the performance, reload and update
// examples use, which is far smaller than the Lite file. The data repository
// links it too.
const AsnDataFile = "51Degrees-IPIV4AsnIpiV41.ipi"

// EvidenceFile holds the addresses the offline processing, performance and
// reload examples read when EVIDENCE_YAML names no other file. The data
// repository has it.
const EvidenceFile = "evidence.yml"

type ExampleParams struct {
	LicenseKey     string
	Product        string
	DataFile       string
	DataFileUrl    string
	IterationCount int
	EvidenceYaml   string
}

type ExampleFunc func(params *ExampleParams) error

// RunExample runs the example with the data file DATA_FILE names, or the free
// Lite data file.
func RunExample(exampleFunc ExampleFunc) {
	RunExampleWithDataFile(LiteDataFile, exampleFunc)
}

// RunExampleWithDataFile runs the example with the data file DATA_FILE names,
// or the free data file given.
func RunExampleWithDataFile(free string, exampleFunc ExampleFunc) {
	licenseKey := os.Getenv("LICENSE_KEY")
	if licenseKey == "" {
		licenseKey = os.Getenv("IPI_KEY")
	}

	dataFile := os.Getenv("DATA_FILE")
	if dataFile == "" {
		dataFile = free
	}

	evidenceYaml := os.Getenv("EVIDENCE_YAML")
	if evidenceYaml == "" {
		evidenceYaml = GetDataFolderFile(EvidenceFile)
	}

	dataFileUrl := os.Getenv("IPI_DATA_FILE_URL")

	params := &ExampleParams{
		LicenseKey:   licenseKey,
		DataFile:     GetDataFile(dataFile),
		DataFileUrl:  dataFileUrl,
		EvidenceYaml: evidenceYaml,
	}

	err := exampleFunc(params)
	if err != nil {
		panic(err)
	}
}
