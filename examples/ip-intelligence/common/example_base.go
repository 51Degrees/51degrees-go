/* *********************************************************************
 * This Original Work is copyright of 51 Degrees Mobile Experts Limited.
 * Copyright 2026 51 Degrees Mobile Experts Limited, Davidson House,
 * Forbury Square, Reading, Berkshire, United Kingdom RG1 3EU.
 *
 * This Original Work is licensed under the European Union Public Licence
 * (EUPL) v.1.2 and is subject to its terms as set out below.
 *
 * If a copy of the EUPL was not distributed with this file, You can obtain
 * one at https://opensource.org/licenses/EUPL-1.2.
 *
 * The 'Compatible Licences' set out in the Appendix to the EUPL (as may be
 * amended by the European Commission) shall be deemed incompatible for
 * the purposes of the Work and the provisions of the compatibility
 * clause in Article 5 of the EUPL shall not apply.
 *
 * If using the Work as, or as part of, a network application, by
 * including the attribution notice(s) required under Article 5 of the EUPL
 * in the end user terms of the application under an appropriate heading,
 * such notice(s) shall fulfill the requirements of that article.
 * ********************************************************************* */

package common

// LiteDataFile is the free data file the examples use when
// _51DEGREES_IPI_PATH names no other. The data repository links it, so an
// example downloads it the first time it runs.
const LiteDataFile = "51Degrees-LiteV41.ipi"

// AsnDataFile is the free data file the performance, reload and update
// examples use, which is far smaller than the Lite file. The data repository
// links it too.
const AsnDataFile = "51Degrees-IPIV4AsnIpiV41.ipi"

// EvidenceFile holds the addresses the offline processing, performance and
// reload examples read when _51DEGREES_EVIDENCE_YAML names no other file.
// The data repository has it.
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

// RunExample runs the example with the data file _51DEGREES_IPI_PATH names,
// or the free Lite data file.
func RunExample(exampleFunc ExampleFunc) {
	RunExampleWithDataFile(LiteDataFile, exampleFunc)
}

// RunExampleWithDataFile runs the example with the data file
// _51DEGREES_IPI_PATH names, or the free data file given.
func RunExampleWithDataFile(free string, exampleFunc ExampleFunc) {
	licenseKey := fromEnvironment("_51DEGREES_LICENSE_KEY")

	dataFile := fromEnvironment("_51DEGREES_IPI_PATH")
	if dataFile == "" {
		dataFile = free
	}

	evidenceYaml := fromEnvironment("_51DEGREES_EVIDENCE_YAML")
	if evidenceYaml == "" {
		evidenceYaml = GetDataFolderFile(EvidenceFile)
	}

	dataFileUrl := fromEnvironment("_51DEGREES_IPI_DATA_FILE_URL")

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
