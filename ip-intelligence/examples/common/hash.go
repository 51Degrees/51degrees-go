package common

import "hash/fnv"

// GenerateHash returns a 32-bit FNV-1 checksum of the input string. It is a
// general-purpose utility used by the examples to compare output snapshots.
func GenerateHash(str string) uint32 {
	h := fnv.New32()
	h.Write([]byte(str))
	return h.Sum32()
}
