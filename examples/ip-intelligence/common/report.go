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
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

// Report struct for each performance run
type Report struct {
	IterationCount    int
	EvidenceCount     uint64
	EvidenceProcessed uint64
	ProcessingTime    int64

	mu        sync.Mutex // Mutex
	HashCodes []uint32
}

func (r *Report) AverageProcessingTime() float64 {
	return float64(r.ProcessingTime) / float64(r.EvidenceProcessed)
}

func (r *Report) DetectionPerSecond() float64 {
	return float64(r.EvidenceProcessed) * 1000 / float64(r.ProcessingTime)
}

func (r *Report) PrintReport(logOutputPath string) error {
	var path string
	if filepath.IsAbs(logOutputPath) {
		path = logOutputPath
	} else {
		rootDir, e := os.Getwd()
		if e != nil {
			log.Fatalln("Failed to get current directory.")
		}
		path = filepath.Join(rootDir, logOutputPath)
	}

	// Create a report file
	f, err := os.Create(path)
	if err != nil {
		log.Fatalf("Failed to create report file \"%s\".", path)
	}
	defer f.Close()

	var reportData bytes.Buffer

	reportData.WriteString(fmt.Sprintf("Average %.5f ms per Evidence Record\n", r.AverageProcessingTime()))
	reportData.WriteString(fmt.Sprintf("Average %.2f detections per second\n", r.DetectionPerSecond()))
	reportData.WriteString(fmt.Sprintf("Total Evidence Records: %d\n", r.EvidenceCount))
	reportData.WriteString(fmt.Sprintf("Iteration Count: %d\n", r.IterationCount))
	reportData.WriteString(fmt.Sprintf("Processed Evidence Records: %d\n", r.EvidenceProcessed))
	reportData.WriteString(fmt.Sprintf("Number of CPUs: %d\n", runtime.NumCPU()))

	if _, err = f.Write(reportData.Bytes()); err != nil {
		return err
	}

	return nil
}

func (r *Report) InitHashCodes(i int) {
	r.HashCodes = make([]uint32, i)
}

// updateHashCode updates the hash code with the input code ad the index
// specified. The update use XOR operation. This function is thread safe to
// make sure multiple threads can update the hash code correctly
func (r *Report) UpdateHashCode(code uint32, i uint32) {
	r.mu.Lock()
	r.HashCodes[i] ^= code
	r.mu.Unlock()
}
