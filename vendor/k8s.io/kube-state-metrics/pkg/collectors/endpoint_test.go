/*
Copyright 2017 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package collectors

import (
	"testing"
	"time"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kube-state-metrics/pkg/collectors/testutils"
	"k8s.io/kube-state-metrics/pkg/options"
)

type mockEndpointStore struct {
	list func() ([]v1.Endpoints, error)
}

func (es mockEndpointStore) List() ([]v1.Endpoints, error) {
	return es.list()
}

func TestEndpointCollector(t *testing.T) {
	// Fixed metadata on type and help text. We prepend this to every expected
	// output so we only have to modify a single place when doing adjustments.
	const metadata = `
		# HELP kube_endpoint_address_available Number of addresses available in endpoint.
		# TYPE kube_endpoint_address_available gauge
		# HELP kube_endpoint_address_not_ready Number of addresses not ready in endpoint
		# TYPE kube_endpoint_address_not_ready gauge
		# HELP kube_endpoint_created Unix creation timestamp
		# TYPE kube_endpoint_created gauge
		# HELP kube_endpoint_info Information about endpoint.
		# TYPE kube_endpoint_info gauge
		# HELP kube_endpoint_labels Kubernetes labels converted to Prometheus labels.
		# TYPE kube_endpoint_labels gauge
	`
	cases := []struct {
		endpoints []v1.Endpoints
		metrics   []string // which metrics should be checked
		want      string
	}{
		{
			endpoints: []v1.Endpoints{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "test-endpoint",
						CreationTimestamp: metav1.Time{Time: time.Unix(1500000000, 0)},
						Namespace:         "default",
						Labels: map[string]string{
							"app": "foobar",
						},
					},
					Subsets: []v1.EndpointSubset{
						{Addresses: []v1.EndpointAddress{
							{IP: "127.0.0.1"}, {IP: "10.0.0.1"},
						},
							Ports: []v1.EndpointPort{
								{Port: 8080}, {Port: 8081},
							},
						},
						{Addresses: []v1.EndpointAddress{
							{IP: "172.22.23.202"},
						},
							Ports: []v1.EndpointPort{
								{Port: 8443}, {Port: 9090},
							},
						},
						{NotReadyAddresses: []v1.EndpointAddress{
							{IP: "192.168.1.1"},
						},
							Ports: []v1.EndpointPort{
								{Port: 1234}, {Port: 5678},
							},
						},
						{NotReadyAddresses: []v1.EndpointAddress{
							{IP: "192.168.1.3"}, {IP: "192.168.2.2"},
						},
							Ports: []v1.EndpointPort{
								{Port: 1234}, {Port: 5678},
							},
						},
					},
				},
			},
			want: metadata + `
				kube_endpoint_address_available{endpoint="test-endpoint",namespace="default"} 6
				kube_endpoint_address_not_ready{endpoint="test-endpoint",namespace="default"} 6
				kube_endpoint_created{endpoint="test-endpoint",namespace="default"} 1.5e+09
				kube_endpoint_info{endpoint="test-endpoint",namespace="default"} 1
				kube_endpoint_labels{endpoint="test-endpoint",label_app="foobar",namespace="default"} 1
			`,
		},
	}
	for _, c := range cases {
		sc := &endpointCollector{
			store: &mockEndpointStore{
				list: func() ([]v1.Endpoints, error) {
					return c.endpoints, nil
				},
			},
			opts: &options.Options{},
		}
		if err := testutils.GatherAndCompare(sc, c.want, c.metrics); err != nil {
			t.Errorf("unexpected collecting result:\n%s", err)
		}
	}
}
