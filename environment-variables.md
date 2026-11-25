# Environment variable names

Every environment variable that 51Degrees defines for customers starts `_51DEGREES_`. This page lists the ones the modules, examples, tests, and CI scripts of this repository read and set, and the two that only CI uses. Each name holds one thing, and no name is filled from another.

The test `TestEveryVariableHasA51DegreesName` in [`common/variable_names_test.go`](common/variable_names_test.go) fails when a tracked file reads or sets a variable that is not in these tables, other than a name that Go, Git, GitHub, PowerShell, or the operating system defines. A name only CI uses must also be in the test's `ciOnly` list. Add a name here in the same change that first reads it.

## Names you set

| Name | Holds | Read by |
| --- | --- | --- |
| `_51DEGREES_RESOURCE_KEY` | Your Resource Key for the 51Degrees cloud service | The cloud examples |
| `_51DEGREES_LICENSE_KEY` | Your License Key for data file updates | The on-premise examples that update a data file, and the CI script that fetches the data files |
| `_51DEGREES_DD_PATH` | The path of a Device Detection data file, in place of the free Lite file | The Device Detection and mixed examples, and their tests |
| `_51DEGREES_DD_PRODUCT` | The product the Device Detection update example asks the Distributor for, such as `V4TAC`. The example names a product only when this is set. When it isn't set, the example sends no product | The Device Detection update polling interval example |
| `_51DEGREES_IPI_PRODUCT` | The product the IP Intelligence update example asks the Distributor for, such as `IPIV4Enterprise`. The example names a product only when this is set. When it isn't set, the example sends no product | The IP Intelligence update polling interval example |
| `_51DEGREES_IPI_PATH` | The path of an IP Intelligence data file, in place of the free Lite file | The IP Intelligence and mixed examples, the IP Intelligence tests, and the performance tests |
| `_51DEGREES_EVIDENCE_YAML` | The path of a YAML evidence file, in place of the free evidence file | The on-premise examples and their tests |
| `_51DEGREES_CLOUD_ENDPOINT` | The address of the cloud service, in place of the public one | The Cloud Request Engine, when `SetEndpoint` is not used, and the Human Confidence example |

## Names the tests read

The test of the IP Intelligence cloud example runs it with `_51DEGREES_RESOURCE_KEY` set to `_51DEGREES_RESOURCE_KEY_BESPOKE`, and requires the values that key gives for a known address. It is skipped when the key is not set. `.github/workflows/cloud-ip-intelligence.yml` sets the key and fails when the test is skipped.

| Name | Holds |
| --- | --- |
| `_51DEGREES_RESOURCE_KEY_BASIC` | A Resource Key on the basic data tier |
| `_51DEGREES_RESOURCE_KEY_BESPOKE` | A Resource Key that includes every property |

The live Human Confidence tests read `_51DEGREES_RESOURCE_KEY_BASIC` as well, against the staging cloud, as the next section says.

## Names only CI uses

The live Human Confidence tests in `examples/pipeline/human_confidence/live_test.go` check what the production cloud does not serve yet, being `HumanConfidenceSource` and a passive level other than `Unknown`, so they call the staging cloud. They read the two names below, with `_51DEGREES_RESOURCE_KEY_BASIC` for a key outside the Bespoke product, and skip when a name is not set. No customer sets these names, so they have no `_51DEGREES_` prefix. common-ci passes on only `_51DEGREES_` names, so only `.github/workflows/cloud-human-confidence.yml` sets them, and that workflow fails when a name is empty or a test is skipped. The tests move back to `_51DEGREES_CLOUD_ENDPOINT` once the production cloud serves these.

| Name | Holds | Kind |
| --- | --- | --- |
| `CLOUD_STAGING_ENDPOINT` | The staging cloud's API base, ending `/api/v4/` | An organization variable |
| `RESOURCE_KEY_BESPOKE_STAGING` | A Resource Key with the Bespoke product that the staging cloud knows | An organization secret |

## Names a test sets for itself

| Name | Holds |
| --- | --- |
| `_51DEGREES_WEB_ETAG_CHILD` | Set by the ETag test for the child process it starts |
