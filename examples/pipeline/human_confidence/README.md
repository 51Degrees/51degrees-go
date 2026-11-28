# Human Confidence example

Human Confidence says how likely it is that a person is looking at a web
page. The program in this folder is a small web server and one page that
show it working, the way a publisher's website would use it. It's a web
demo rather than a console program, because the browser tests only run in a
browser. It uses the Pipeline API's Cloud Request Engine and Human Confidence
Engine.

## The flow

The page shows three rows, one for each step. Each row shows the two
Properties `HumanConfidenceSource` and `HumanConfidence`, or the reason the
Pipeline gives where one has no value.

1. **Passive.** The server processes the visitor's request headers (only
   those the cloud lists in its `evidencekeys`) and IP address through its
   Pipeline, and reads the two Properties.
2. **Active.** For each page it serves, the server asks its Cloud Request
   Engine for a new page (`IssueHumanConfidencePage`). The page loads the
   51Degrees client script from that page's address, which carries a
   transaction the Pipeline keeps on the server, so the sealed value the
   page gets is bound to it. When the script completes,
   `HumanConfidenceSource` is `Active` and `HumanConfidence` holds a sealed
   value. The browser can't read it or change it.
3. **Read with the sealed result.** The page sends the sealed value back to
   the server as `humanconfidence`, with its page id as
   `humanconfidencepage`, which the server adds as the Evidence
   `query.humanconfidence` and `query.humanconfidencepage`, and the server
   reads the two Properties again. Its Pipeline redeems the sealed value
   through the Cloud Request Engine's request, with the account's License
   Key and the page's transaction, so the server never redeems anything
   itself. `HumanConfidenceSource` is `Active`, and `HumanConfidence` is the
   redeemed Value. A sealed value redeems once, within ten seconds. Where it
   can't be redeemed, for any reason, `HumanConfidence` is `Medium`. The
   page then offers to send the same value again, which gives `Medium`,
   because the page's transaction has been used.

The client script reports complete only after the browser tests have run
and one more request has answered, so the second row fills a few seconds
after the first. A page can read the first answer's Properties sooner
through the client script's change handlers.

Where a page sets a Content Security Policy, it must allow these for the
browser tests to run. Otherwise `HumanConfidenceSource` stays `Passive`.

| Directive | Must allow |
| --- | --- |
| `script-src` | The cloud's host, and `'unsafe-eval'` |
| `connect-src` | The cloud's host |
| `worker-src` | `blob:` |

## What you need

- **A Resource Key** with the properties `HumanConfidence` and
  `HumanConfidenceSource`. When the key lacks `HumanConfidence`, the server
  and the page say so, with a link to
  [contact us](https://51degrees.com/contact-us?utm_source=github&utm_medium=readme&utm_campaign=51degrees-go&utm_content=human-confidence&utm_term=what-you-need).
- **Leave the Resource Key's domain list empty,** or include `localhost`
  and the server's own address. The server calls the cloud with the same
  key, and a call from a server has no `Origin` header.
- **A License Key** of the same account. The server's Pipeline uses it to redeem. It
  never goes in the page.

## Environment variables

| Variable | Meaning |
| --- | --- |
| `_51DEGREES_RESOURCE_KEY` | The Resource Key. Required. |
| `_51DEGREES_LICENSE_KEY` | A License Key of the same account. Server side only. Needed to redeem. |
| `_51DEGREES_CLOUD_ENDPOINT` | Optional. The API base including the `/api/v4/` segment. It defaults to `https://cloud.51degrees.com/api/v4/`. Set it to use another address. |
| `PORT` | The port to listen on. It defaults to `5151`. |

## How to run

From `examples/pipeline`, in PowerShell:

```
$env:_51DEGREES_RESOURCE_KEY = "your Resource Key"
$env:_51DEGREES_LICENSE_KEY = "your License Key"
go run ./human_confidence
```

Or in a bash shell:

```
export _51DEGREES_RESOURCE_KEY="your Resource Key"
export _51DEGREES_LICENSE_KEY="your License Key"
go run ./human_confidence
```

Then open `http://localhost:5151/` in a browser.

Apart from the live tests below, the tests use a fake cloud and need no keys.

```
go test ./human_confidence
```

The tests whose names start `TestLive` make a passive call like the
example's against the staging cloud, because the production cloud doesn't
serve `HumanConfidenceSource` or a passive level other than `Unknown` yet.
`CLOUD_STAGING_ENDPOINT` names the staging cloud, with no default. With the
key in `RESOURCE_KEY_BESPOKE_STAGING` the answer has `HumanConfidenceSource`
set to `Passive` and a plain level. With the key in
`_51DEGREES_RESOURCE_KEY_BASIC`, which is outside the Bespoke product, the
answer names `HumanConfidenceSource` as null, with a reason. Only CI sets
the two names without the `_51DEGREES_` prefix, in
`.github/workflows/cloud-human-confidence.yml`, and each test skips when a
name it needs isn't set. The active source comes back only to a
browser that has run the tests, so these tests don't check it.

## What you should see

1. The first row shows `Passive` and a level.
2. After a few seconds, the second row shows `Active` and a sealed value.
3. The third row shows `Active` and the redeemed Value.
4. **Send the same sealed result again** shows `Active, Medium`.

Results vary by browser and device. A sealed value sent more than ten
seconds after the tests gives `Medium`. Without a License Key, the third row
shows `Medium`.

If the second row stays `Passive`, the browser tests didn't run. The cloud
you're using may not have the tests yet, or the Resource Key may not
include `HumanConfidenceSource`.

## What a run costs

Every call to the cloud is one use against the subscription behind the
keys. A page load makes several, being the server's passive call, the
client script's calls, and the server's call with the sealed result.

## Find out more

- [Documentation](https://51degrees.com/documentation/?utm_source=github&utm_medium=readme&utm_campaign=51degrees-go&utm_content=human-confidence&utm_term=find-out-more)
- [Pricing](https://51degrees.com/pricing?utm_source=github&utm_medium=readme&utm_campaign=51degrees-go&utm_content=human-confidence&utm_term=find-out-more)
- [Contact us](https://51degrees.com/contact-us?utm_source=github&utm_medium=readme&utm_campaign=51degrees-go&utm_content=human-confidence&utm_term=find-out-more)
