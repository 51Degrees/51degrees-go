# Environment variable names

Every environment variable that 51Degrees defines starts `_51DEGREES_`. This page lists the ones the modules, examples, tests, and CI scripts of this repository read and set. Each name holds one thing, and no name is filled from another.

The test `TestEveryVariableHasA51DegreesName` in [`common/variable_names_test.go`](common/variable_names_test.go) fails when a tracked file reads or sets a variable that is not in these tables, other than a name that Go, Git, GitHub, PowerShell, or the operating system defines. Add a name here in the same change that first reads it.

## Names you set

| Name | Holds | Read by |
| --- | --- | --- |
| `_51DEGREES_RESOURCE_KEY` | Your Resource Key for the 51Degrees cloud service | The cloud examples |
| `_51DEGREES_LICENSE_KEY` | Your License Key for data file updates | The on-premise examples that update a data file, and the CI script that fetches the data files |
| `_51DEGREES_DD_PATH` | The path of a Device Detection data file, in place of the free Lite file | The Device Detection and mixed examples, and their tests |
| `_51DEGREES_DD_PRODUCT` | The product the Device Detection update example asks the Distributor for, such as `V4TAC`. When it is not set, the example names no product | The Device Detection update polling interval example |
| `_51DEGREES_IPI_PATH` | The path of an IP Intelligence data file, in place of the free Lite file | The IP Intelligence and mixed examples, the IP Intelligence tests, and the performance tests |
| `_51DEGREES_EVIDENCE_YAML` | The path of a YAML evidence file, in place of the free evidence file | The on-premise examples and their tests |
| `_51DEGREES_CLOUD_ENDPOINT` | The address of the cloud service, in place of the public one | The Cloud Request Engine, when `SetEndpoint` is not used |

## Names the tests read

The tests of the cloud examples run each example once for each of these keys that is set, with `_51DEGREES_RESOURCE_KEY` set to that key. A test whose keys are all unset is skipped.

| Name | Holds |
| --- | --- |
| `_51DEGREES_RESOURCE_KEY_FREE` | A Resource Key on the free data tier |
| `_51DEGREES_RESOURCE_KEY_BASIC` | A Resource Key on the basic data tier |
| `_51DEGREES_RESOURCE_KEY_BESPOKE` | A Resource Key that includes every property |

## Names a test sets for itself

| Name | Holds |
| --- | --- |
| `_51DEGREES_WEB_ETAG_CHILD` | Set by the ETag test for the child process it starts |
