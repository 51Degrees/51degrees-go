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
