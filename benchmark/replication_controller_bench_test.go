package benchmark

import (
	"testing"
	"time"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/test/integration/framework"
)

// BenchmarkRC100Nodes0Pods benchmarks the rc rate
// when the cluster has 100 nodes and 0 scheduled pods
func BenchmarkRC100Nodes0RC0Pods(b *testing.B) {
	benchmarkRC(100, 0, 0, b)
}

// BenchmarkScheduling100Nodes1000Pods benchmarks the scheduling rate
// when the cluster has 100 nodes and 1000 scheduled pods
func BenchmarkRC100Nodes10RC1000Pods(b *testing.B) {
	benchmarkRC(100, 10, 100, b)
}

func BenchmarkRC100Nodes100RC3000Pods(b *testing.B) {
	benchmarkRC(100, 100, 30, b)
}

// BenchmarkScheduling1000Nodes0Pods benchmarks the scheduling rate
// when the cluster has 1000 nodes and 0 scheduled pods
func BenchmarkRC1000Nodes0RC0Pods(b *testing.B) {
	benchmarkRC(1000, 0, 0, b)
}

// BenchmarkScheduling1000Nodes10000Pods benchmarks the scheduling rate
// when the cluster has 1000 nodes and 1000 scheduled pods
func BenchmarkRC1000Nodes10RC1000Pods(b *testing.B) {
	benchmarkRC(1000, 10, 100, b)
}

func BenchmarkRC1000Nodes100RC3000Pods(b *testing.B) {
	benchmarkRC(1000, 100, 30, b)
}

func BenchmarkRC1000Nodes500RC10000Pods(b *testing.B) {
	benchmarkRC(1000, 500, 20, b)
}

func benchmarkRC(n, r, p int, b *testing.B) {
	m, finalFunc := mustSetupController()
	defer finalFunc()

	makeNodes(m.RestClient, n)
	numPods := r * p
	makeRCPods(m.RestClient, r, p)
	for {
		created := m.RestClient.Pods("default").List(api.ListOptions{})
		if len(created.Items) >= numPods {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	// start benchmark
	b.ResetTimer()
	makeRCPods(m.RestClient, 1, 2000)
	for {
		created := m.RestClient.Pods("default").List(api.ListOptions{})
		if len(created.Items) >= numPods+2000 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	b.StopTimer()
}
