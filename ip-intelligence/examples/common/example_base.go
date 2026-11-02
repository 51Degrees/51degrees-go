package common

import (
	"os"
)

type ExampleParams struct {
	LicenseKey     string
	Product        string
	DataFile       string
	DataFileUrl    string
	IterationCount int
	EvidenceYaml   string
}

type ExampleFunc func(params *ExampleParams) error

func RunExample(exampleFunc ExampleFunc) {
	licenseKey := os.Getenv("LICENSE_KEY")
	if licenseKey == "" {
		licenseKey = os.Getenv("IPI_KEY")
	}

	dataFile := os.Getenv("DATA_FILE")
	if dataFile == "" {
		dataFile = "51Degrees-EnterpriseIpiV41.ipi"
	}

	evidenceYaml := os.Getenv("EVIDENCE_YAML")
	if evidenceYaml == "" {
		evidenceYaml = "20000_ipi_evidence_records.yml"
	}

	dataFileUrl := os.Getenv("IPI_DATA_FILE_URL")

	params := &ExampleParams{
		LicenseKey:   licenseKey,
		DataFile:     dataFile,
		DataFileUrl:  dataFileUrl,
		EvidenceYaml: evidenceYaml,
	}

	err := exampleFunc(params)
	if err != nil {
		panic(err)
	}
}
