#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

source "./hack/lib/util.sh"
source "./hack/lib/logging.sh"
source "./hack/lib/etcd.sh"

cleanup() {
  kube::etcd::cleanup
  kube::log::status "Integration test cleanup complete"
}

trap cleanup EXIT

kube::log::status "Downloading dependencies..."
# download the necessary dependencies for testing
go get -t -d

kube::etcd::start
kube::log::status "Start benchmarking..."

# TODO: set log-dir and remove it after benchmark
go test -c
./benchmark.test -test.bench=. -test.run=xxxx -test.cpuprofile=prof.out -logtostderr=false
