FROM registry.access.redhat.com/ubi9/ubi:latest AS builder
WORKDIR /go/src/openshift-eng/ci-test-mapping
ENV PATH="/go/bin:${PATH}"
ENV GOPATH="/go"
RUN dnf install -y \
        git \
        go \
        make \
     && go install github.com/Link-/gh-token@latest \
                   k8s.io/test-infra/robots/pr-creator@latest
COPY . .
RUN make build

FROM registry.access.redhat.com/ubi9/ubi:latest AS base
RUN dnf install -y git jq
COPY --from=builder /go/src/openshift-eng/ci-test-mapping/ci-test-mapping /bin/ci-test-mapping
COPY --from=builder /go/bin/gh-token /bin/gh-token
COPY --from=builder /go/bin/pr-creator /bin/pr-creator
COPY hack /hack
ENTRYPOINT ["/bin/ci-test-mapping"]
