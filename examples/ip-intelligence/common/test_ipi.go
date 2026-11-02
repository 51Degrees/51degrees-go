package common

type TestIpi struct {
	IpAddress        string
	Expected         string
	IsWorkingExample bool // This parameter indicates whether the error received is expected or not.
}
