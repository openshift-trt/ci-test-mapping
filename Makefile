all: test build

verify: lint

build:
	go build .

test:
	go test ./...

mapping: build
	# OCP Engineering
	./ci-test-mapping map --mode=local
	# QE
	./ci-test-mapping map --bigquery-dataset ci_analysis_us --bigquery-project openshift-gce-devel --bigquery-dataset ci_analysis_qe --table-junit junit --table-mapping component_mapping --mode=local --config ""
	# Verify mappings don't move anything into Unknown
	./ci-test-mapping map-verify

unmapped:
	jq -r '.[] | select(.Component == "Unknown") | .Name' mapping.json | sort | uniq

lint:
	./hack/go-lint.sh run ./...

clean:
	rm -f ci-test-mapping
