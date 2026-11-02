// The mixed examples intentionally combine 51Degrees Device Detection and
// IP Intelligence. They live in their own module so that the root
// ip-intelligence-go module does not depend on device-detection-go.
// This module is never published; the replace directive resolves the
// IP Intelligence dependency to the enclosing repository.
module github.com/51Degrees/51degrees-go/ip-intelligence/examples/mixed

go 1.21.0

require (
	github.com/51Degrees/51degrees-go/device-detection/v4 v4.5.29
	github.com/51Degrees/51degrees-go/ip-intelligence/v4 v4.5.171
)

require github.com/51Degrees/common-go/v4 v4.5.1 // indirect

replace github.com/51Degrees/51degrees-go/ip-intelligence/v4 => ../..

replace github.com/51Degrees/51degrees-go/device-detection/v4 => ../../../device-detection
