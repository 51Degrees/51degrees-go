package common

type IpEvidences []string

func (e *IpEvidences) Add(s string) {
	*e = append(*e, s)
}

func (e IpEvidences) Size() uint64 {
	return uint64(len(e))
}
