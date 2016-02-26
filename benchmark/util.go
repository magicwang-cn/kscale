package benchmark

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"

	"github.com/golang/glog"
	controller "k8s.io/kubernetes/cmd/kube-controller-manager/app"
	controller_options "k8s.io/kubernetes/cmd/kube-controller-manager/app/options"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
	"k8s.io/kubernetes/pkg/api/testapi"
	"k8s.io/kubernetes/pkg/client/record"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/master"
	"k8s.io/kubernetes/plugin/pkg/scheduler"
	_ "k8s.io/kubernetes/plugin/pkg/scheduler/algorithmprovider"
	"k8s.io/kubernetes/plugin/pkg/scheduler/factory"
	"k8s.io/kubernetes/test/integration/framework"
)

var (
	podSpec = api.PodSpec{
		Containers: []api.Container{{
			Name:  "pause",
			Image: "gcr.io/google_containers/pause:1.0",
			Resources: api.ResourceRequirements{
				Limits: api.ResourceList{
					api.ResourceCPU:    resource.MustParse("100m"),
					api.ResourceMemory: resource.MustParse("500Mi"),
				},
				Requests: api.ResourceList{
					api.ResourceCPU:    resource.MustParse("100m"),
					api.ResourceMemory: resource.MustParse("500Mi"),
				},
			},
		}},
	}
)

func mustSetupController() *framework.MasterComponents {
	config := &framework.Config{
		MasterConfig:            framework.NewIntegrationTestMasterConfig(),
		StartReplicationManager: true,
		DeleteEtcdKeys:          true,
		QPS:                     5000.0,
		Burst:                   5000,
	}
	m := framework.NewMasterComponents(config)

	destroyFunc = func() {
		glog.Infof("destroying")
		m.Stop()
		glog.Infof("destroyed")
	}

	return m, destroyFunc
}

// rate limiting notes:
//   - The BindPodsRateLimiter is nil, basically not rate limits.
//   - client rate limit is set to 5000.
func mustSetupScheduler() (schedulerConfigFactory *factory.ConfigFactory, destroyFunc func()) {
	framework.DeleteAllEtcdKeys()

	config := createConfig()
	c, s := createClient(config)

	schedulerConfigFactory = factory.NewConfigFactory(c, nil)
	schedulerConfig, err := schedulerConfigFactory.Create()
	if err != nil {
		panic("Couldn't create scheduler config")
	}
	eventBroadcaster := record.NewBroadcaster()
	schedulerConfig.Recorder = eventBroadcaster.NewRecorder(api.EventSource{Component: "scheduler"})
	eventBroadcaster.StartRecordingToSink(c.Events(""))
	scheduler.New(schedulerConfig).Run()

	destroyFunc = func() {
		glog.Infof("destroying")
		close(schedulerConfig.StopEverything)
		s.Close()
		glog.Infof("destroyed")
	}
	return
}

func createClient(config *client.Config) (*client.Client, *http.Server) {
	var m *master.Master
	masterConfig := framework.NewIntegrationTestMasterConfig()
	m = master.New(masterConfig)
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		m.Handler.ServeHTTP(w, req)
	}))

	return client.NewOrDie(config), s
}

func createConfig() *client.Config {
	return &client.Config{
		Host:         s.URL,
		GroupVersion: testapi.Default.GroupVersion(),
		QPS:          5000.0,
		Burst:        5000,
	}
}

func makeNodes(c client.Interface, nodeCount int) {
	glog.Infof("making %d nodes", nodeCount)
	baseNode := &api.Node{
		ObjectMeta: api.ObjectMeta{
			GenerateName: "scheduler-test-node-",
		},
		Spec: api.NodeSpec{
			ExternalID: "foobar",
		},
		Status: api.NodeStatus{
			Capacity: api.ResourceList{
				api.ResourcePods:   *resource.NewQuantity(32, resource.DecimalSI),
				api.ResourceCPU:    resource.MustParse("4"),
				api.ResourceMemory: resource.MustParse("32Gi"),
			},
			Phase: api.NodeRunning,
			Conditions: []api.NodeCondition{
				{Type: api.NodeReady, Status: api.ConditionTrue},
			},
		},
	}
	for i := 0; i < nodeCount; i++ {
		if _, err := c.Nodes().Create(baseNode); err != nil {
			panic("error creating node: " + err.Error())
		}
	}
}

func makePods(c client.Interface, podCount int) {
	glog.Infof("making %d pods", podCount)
	basePod := &api.Pod{
		ObjectMeta: api.ObjectMeta{
			GenerateName: "scheduler-test-pod-",
		},
		Spec: podSpec,
	}
	var wg sync.WaitGroup
	threads := 30
	wg.Add(threads)
	remaining := make(chan int, 100)
	go func() {
		for i := 0; i < podCount; i++ {
			remaining <- i
		}
		close(remaining)
	}()
	for i := 0; i < threads; i++ {
		go func() {
			defer wg.Done()
			for {
				_, ok := <-remaining
				if !ok {
					return
				}
				for {
					_, err := c.Pods("default").Create(basePod)
					if err == nil {
						break
					}
				}
			}
		}()
	}
	wg.Wait()
}

func makeRCPods(c client.Interface, rcCount, podsPerRC int) {
	wg := sync.WaitGroup{}
	wg.Add(rcCount)
	for i := 0; i < rcCount; i++ {
		go func(i int) {
			defer wg.Done()
			name := fmt.Sprintf("test-controller-%d", i)
			rc := &api.ReplicationController{
				ObjectMeta: api.ObjectMeta{
					Name: name,
				},
				Spec: api.ReplicationControllerSpec{
					Replicas: podsPerRC,
					Selector: map[string]string{
						"name": name,
					},
					Template: &api.PodTemplateSpec{
						ObjectMeta: api.ObjectMeta{
							Labels: map[string]string{"name": name},
						},
						Spec: api.PodSpec{
							Containers: []api.Container{
								{
									Name:  name,
									Image: "gcr.io/google_containers/pause:1.0",
									Ports: []api.ContainerPort{{ContainerPort: 80}},
								},
							},
							DNSPolicy: api.DNSDefault,
						},
					},
				},
			}

			for {
				_, err := c.ReplicationControllers("default").Create(rc)
				if err == nil {
					break
				}
			}
		}(i)
	}
	wg.Wait()
}
