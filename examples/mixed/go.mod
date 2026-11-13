// The mixed examples combine Device Detection and IP Intelligence. They aren't
// published. Copied out of the repository they use the published modules, as a
// customer's code does. In a clone, the go.work at the root makes them use the
// ones in this repository.
module github.com/51Degrees/51degrees-go/examples/mixed

go 1.22

require (
	github.com/51Degrees/51degrees-go/device-detection/v4 v4.5.173
	github.com/51Degrees/51degrees-go/ip-intelligence/v4 v4.5.173
)

require github.com/51Degrees/51degrees-go/common/v4 v4.5.173 // indirect
