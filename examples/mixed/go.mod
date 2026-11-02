// The mixed examples combine Device Detection and IP Intelligence. They aren't
// published. Like a customer's code, they use the published modules;
// ci/create-workspace.ps1 makes them use the ones in this repository.
module github.com/51Degrees/51degrees-go/examples/mixed

go 1.21.0

require (
	github.com/51Degrees/51degrees-go/common/v4 v4.5.166
	github.com/51Degrees/51degrees-go/device-detection/v4 v4.5.166
	github.com/51Degrees/51degrees-go/ip-intelligence/v4 v4.5.166
)
