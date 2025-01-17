# Component Readiness Test Mapping

Component Readiness needs to know how to map each test to a particular
component and it's capabilities. This tool:

1. Takes a set of metadata about all tests such as it's name and suite
2. Maps the test to exactly one component that provides details about the capabilities that test is testing
3. Writes the result to a mapping.json file comitted to this repo
4. Pushes the result to BigQuery

Teams own their component code under `pkg/<component_name>` and can
handle the mapping as they see fit. New components can copy from
`pkg/example` and modify it, or write their own implementation of the
interface. The example tracks ownership and capabilities using the most
common filters such as sigs, `[Feature:XYZ]` annotations in test names,
as well as test substrings.

Component owners should return a `TestOwnership` struct from their
identification function. See the details in `pkg/api/v1` for details
about the `TestOwnership` struct.

They should return nil when the test is not theirs.  They should ONLY
return an error on a fatal error such as inability to read from a file.

A test should only map to one component, but may map to several
capabilities.  In the event that two components are vying for a test's
ownership, and one wants to force the matter, you may use the `Priority`
field in the `TestOwnership` struct.  The highest value wins.

## Renaming tests

The unfortunate reality is tests may get renamed, so we need to have a
way to compare the test results across renames. To do that, each test
has a stable ID which is currently `test suite + . + test name`, which we
save in the DB as an md5sum.

The first stable ID a test has is the one that remains. Component owners are
responsible for ensuring the `StableID` function in their component
returns the same ID for all names of a given test. This can be done with
a simple look-up map, see the networking component for an example.

# Test Sources

Currently the tests we use for mapping comes from the corpus of tests
we've previously seen in job results. This list is filtered down to
smaller quantity by selecting only those in certain suites. It is
possible to extend or replace this with other data sources, such as
importing JSON files from other repos that includes more metadata than
just test and suite.

At a mimimum though, for compatibility with component readiness, a
test must:

* always have a result when it runs, indicating it's success, flake or failure (historically some tests only report failure)

* belong to a test suite

* must have stable names: do not use dynamic names such as full pod names in tests

* have a reasonable way to map to component/capabilities, such as `[sig-XYZ]` present in the test name, and using `[Feature:XYZ]` or `[Capability:XYZ]` to make mapping to capabilities easier

# Usage

See --help for more info.

## Test Mapping

### Development

For production usage we fetch and push data to BigQuery, but for local
testing you can used locally comitted copies of that data by using
`--mode local`:

```
ci-test-mapping map --mode local
```

### Production

For production, use `--mode bigquery` and provide credentials:

```
ci-test-mapping map --mode bigquery \
  --google-service-account-credential-file ~/bq.json \
  --log-level debug \
  --mapping-file mapping.json \
  --push-to-bigquery
```


### Using the BigQuery table for lookups

The BigQuery mapping table may have older entries trimmed, but it should
be assumed to be used in append only mode, so mappings should limit
their results to the most recent entry.

## Syncing with Jira

To create any missing components, run `./ci-test-mapping create`.
You'll need to set the env var `JIRA_TOKEN` to your personal API token
that you can create from your Jira profile page.
