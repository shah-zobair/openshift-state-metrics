package collectors

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kube-state-metrics/pkg/metrics"

	"github.com/golang/glog"
	"github.com/openshift/api/quota/v1"
	quotaclient "github.com/openshift/client-go/quota/clientset/versioned"
	"k8s.io/kube-state-metrics/pkg/version"
)

var (
	descClusterResourceQuotaLabelsName          = "openshift_clusterresourcequota_labels"
	descClusterResourceQuotaLabelsHelp          = "Kubernetes labels converted to Prometheus labels."
	descClusterResourceQuotaLabelsDefaultLabels = []string{"name"}

	quotaMetricFamilies = []metrics.FamilyGenerator{
		metrics.FamilyGenerator{
			Name: "openshift_clusterresourcequota_created",
			Type: metrics.MetricTypeGauge,
			Help: "Unix creation timestamp",
			GenerateFunc: wrapClusterResourceQuotaFunc(func(b *v1.ClusterResourceQuota) metrics.Family {
				f := metrics.Family{}

				if !b.CreationTimestamp.IsZero() {
					f = append(f, &metrics.Metric{
						Name:  "openshift_clusterresourcequota_created",
						Value: float64(b.CreationTimestamp.Unix()),
					})
				}

				return f
			}),
		},
		metrics.FamilyGenerator{
			Name: "",
			Type: metrics.MetricTypeGauge,
			Help: descClusterResourceQuotaLabelsHelp,
			GenerateFunc: wrapClusterResourceQuotaFunc(func(quota *v1.ClusterResourceQuota) metrics.Family {
				labelKeys, labelValues := kubeLabelsToPrometheusLabels(quota.Labels)
				return metrics.Family{&metrics.Metric{
					Name:        descClusterResourceQuotaLabelsName,
					LabelKeys:   labelKeys,
					LabelValues: labelValues,
					Value:       1,
				}}
			}),
		},
		metrics.FamilyGenerator{
			Name: "openshift_clusterresourcequota",
			Type: metrics.MetricTypeGauge,
			Help: "Information about resource quota.",
			GenerateFunc: wrapClusterResourceQuotaFunc(func(r *v1.ClusterResourceQuota) metrics.Family {
				f := metrics.Family{}

				for res, qty := range r.Status.Total.Hard {
					f = append(f, &metrics.Metric{
						LabelValues: []string{string(res), "hard"},
						Value:       float64(qty.MilliValue()) / 1000,
					})
				}
				for res, qty := range r.Status.Total.Used {
					f = append(f, &metrics.Metric{
						LabelValues: []string{string(res), "used"},
						Value:       float64(qty.MilliValue()) / 1000,
					})
				}

				for _, m := range f {
					m.Name = "openshift_clusterresourcequota"
					m.LabelKeys = []string{"resource", "type"}
				}

				return f
			}),
		},
	}
)

func wrapClusterResourceQuotaFunc(f func(config *v1.ClusterResourceQuota) metrics.Family) func(interface{}) metrics.Family {
	return func(obj interface{}) metrics.Family {
		quota := obj.(*v1.ClusterResourceQuota)

		metricFamily := f(quota)

		for _, m := range metricFamily {
			m.LabelKeys = append(descClusterResourceQuotaLabelsDefaultLabels, m.LabelKeys...)
			m.LabelValues = append([]string{quota.Name}, m.LabelValues...)
		}

		return metricFamily
	}
}

func createClusterResourceQuotaListWatch(apiserver string, kubeconfig string, ns string) cache.ListWatch {
	quotaclient, err := createClusterResourceQuotaClient(apiserver, kubeconfig)
	if err != nil {
		glog.Fatalf("cannot create quota client: %v", err)
	}
	return cache.ListWatch{
		ListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {
			return quotaclient.QuotaV1().ClusterResourceQuotas().List(opts)
		},
		WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
			return quotaclient.QuotaV1().ClusterResourceQuotas().Watch(opts)
		},
	}
}

func createClusterResourceQuotaClient(apiserver string, kubeconfig string) (*quotaclient.Clientset, error) {
	config, err := clientcmd.BuildConfigFromFlags(apiserver, kubeconfig)
	if err != nil {
		return nil, err
	}

	config.UserAgent = version.GetVersion().String()
	config.AcceptContentTypes = "application/vnd.kubernetes.protobuf,application/json"
	config.ContentType = "application/vnd.kubernetes.protobuf"

	client, err := quotaclient.NewForConfig(config)
	return client, err

}
