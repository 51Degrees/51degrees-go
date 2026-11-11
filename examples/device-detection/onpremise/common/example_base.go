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

import (
	"os"
	"strings"

	"github.com/51Degrees/51degrees-go/device-detection/v4/dd"
	"github.com/51Degrees/51degrees-go/device-detection/v4/onpremise"
	dd_example "github.com/51Degrees/51degrees-go/examples/device-detection/dd"
)

type ExampleParams struct {
	LicenseKey   string
	Product      string
	DataFile     string
	EvidenceYaml string
}

type ExampleFunc func(params ExampleParams) error

func RunExample(exampleFunc ExampleFunc) {
	licenseKey := os.Getenv("_51DEGREES_LICENSE_KEY")

	dataFile := os.Getenv("_51DEGREES_DD_PATH")
	if dataFile == "" {
		dataFile = "51Degrees-LiteV4.1.hash"
	}

	evidenceYaml := os.Getenv("_51DEGREES_EVIDENCE_YAML")
	if evidenceYaml == "" {
		evidenceYaml = "20000 Evidence Records.yml"
	}

	params := ExampleParams{
		LicenseKey:   licenseKey,
		DataFile:     dd_example.GetDataFile(dataFile),
		EvidenceYaml: evidenceYaml,
	}

	err := exampleFunc(params)
	if err != nil {
		panic(err)
	}
}

var ExampleEvidenceDesktop = []onpremise.Evidence{
	{Prefix: dd.HttpHeaderString, Key: "Sec-Ch-Ua", Value: "\"Chromium\";v=\"124\", \"Google Chrome\";v=\"124\", \"Not-A.Brand\";v=\"99\""},
	{Prefix: dd.HttpHeaderString, Key: "Sec-Ch-Ua-Full-Version-List", Value: "\"Chromium\";v=\"124.0.6367.208\", \"Google Chrome\";v=\"124.0.6367.208\", \"Not-A.Brand\";v=\"99.0.0.0\""},
	{Prefix: dd.HttpHeaderString, Key: "Sec-Ch-Ua-Mobile", Value: "?0"},
	{Prefix: dd.HttpHeaderString, Key: "Sec-Ch-Ua-Platform", Value: "\"macOS\""},
	{Prefix: dd.HttpHeaderString, Key: "Sec-Ch-Ua-Platform-Version", Value: "\"14.4.1\""},
	{Prefix: dd.HttpHeaderString, Key: "User-Agent", Value: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"},
}

var ExampleEvidenceMobile = []onpremise.Evidence{
	{Prefix: dd.HttpHeaderString, Key: "User-Agent", Value: "Mozilla/5.0 (iPhone; CPU iPhone OS 17_1_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.1.2 Mobile/15E148 Safari/604.1"},
}

var ExampleEvidenceMediaHub = []onpremise.Evidence{
	{Prefix: dd.HttpHeaderString, Key: "User-Agent", Value: "Mozilla/5.0 (Linux; U; Android 4.4.2; en-us; A464BG Build/KOT49H) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Mobile Safari/537.36"},
}

func ConvertToEvidence(values map[string]string) []onpremise.Evidence {
	evidence := make([]onpremise.Evidence, len(values))
	for k, v := range values {
		strSplit := strings.SplitN(k, ".", 2)
		var prefix dd.EvidencePrefix
		switch strSplit[0] {
		case "header":
			prefix = dd.HttpHeaderString
		case "query":
			prefix = dd.HttpEvidenceQuery
		}

		evidence = append(evidence,
			onpremise.Evidence{
				Prefix: prefix,
				Key:    strSplit[1],
				Value:  v,
			})
	}
	return evidence
}

func GetEvidenceUserAgent(evidence []onpremise.Evidence) string {
	for i := range evidence {
		if evidence[i].Key == "User-Agent" {
			return evidence[i].Value
		}
	}
	return ""
}
