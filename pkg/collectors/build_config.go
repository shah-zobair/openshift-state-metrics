package collectors

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kube-state-metrics/pkg/metrics"
	"k8s.io/kube-state-metrics/pkg/version"

	"github.com/golang/glog"

	"github.com/openshift/api/build/v1"
	buildclient "github.com/openshift/client-go/build/clientset/versioned"
)

var (
	descBuildConfigLabelsName          = "openshift_buildconfig_labels"
	descBuildConfigLabelsHelp          = "Kubernetes labels converted to Prometheus labels."
	descBuildConfigLabelsDefaultLabels = []string{"namespace", "buildconfig"}

	buildconfigMetricFamilies = []metrics.FamilyGenerator{
		metrics.FamilyGenerator{
			Name: "openshift_buildconfig_created",
			Type: metrics.MetricTypeGauge,
			Help: "Unix creation timestamp",
			GenerateFunc: wrapBuildConfigFunc(func(d *v1.BuildConfig) metrics.Family {
				f := metrics.Family{}

				if !d.CreationTimestamp.IsZero() {
					f = append(f, &metrics.Metric{
						Name:  "openshift_buildconfig_created",
						Value: float64(d.CreationTimestamp.Unix()),
					})
				}

				return f
			}),
		},
		metrics.FamilyGenerator{
			Name: "openshift_buildconfig_metadata_generation",
			Type: metrics.MetricTypeGauge,
			Help: "Sequence number representing a specific generation of the desired state.",
			GenerateFunc: wrapBuildConfigFunc(func(d *v1.BuildConfig) metrics.Family {
				return metrics.Family{&metrics.Metric{
					Name:  "openshift_buildconfig_metadata_generation",
					Value: float64(d.ObjectMeta.Generation),
				}}
			}),
		},
		metrics.FamilyGenerator{
			Name: descBuildConfigLabelsName,
			Type: metrics.MetricTypeGauge,
			Help: descBuildConfigLabelsHelp,
			GenerateFunc: wrapBuildConfigFunc(func(d *v1.BuildConfig) metrics.Family {
				labelKeys, labelValues := kubeLabelsToPrometheusLabels(d.Labels)
				return metrics.Family{&metrics.Metric{
					Name:        descBuildConfigLabelsName,
					LabelKeys:   labelKeys,
					LabelValues: labelValues,
					Value:       1,
				}}
			}),
		},
		metrics.FamilyGenerator{
			Name: "openshift_buildconfig_status_latest_version",
			Type: metrics.MetricTypeGauge,
			Help: "The latest version of buildconfig.",
			GenerateFunc: wrapBuildConfigFunc(func(d *v1.BuildConfig) metrics.Family {
				return metrics.Family{&metrics.Metric{
					Name:  "openshift_buildconfig_status_latest_version",
					Value: float64(d.Status.LastVersion),
				}}
			}),
		},
	}
)

func wrapBuildConfigFunc(f func(config *v1.BuildConfig) metrics.Family) func(interface{}) metrics.Family {
	return func(obj interface{}) metrics.Family {
		buildconfig := obj.(*v1.BuildConfig)

		metricFamily := f(buildconfig)

		for _, m := range metricFamily {
			m.LabelKeys = append(descBuildConfigLabelsDefaultLabels, m.LabelKeys...)
			m.LabelValues = append([]string{buildconfig.Namespace, buildconfig.Name}, m.LabelValues...)
		}

		return metricFamily
	}
}

func createBuildConfigListWatch(apiserver string, kubeconfig string, ns string) cache.ListWatch {
	buildclient, err := createBuildClient(apiserver, kubeconfig)
	if err != nil {
		glog.Fatalf("cannot create buildconfig client: %v", err)
	}
	return cache.ListWatch{
		ListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {
			return buildclient.BuildV1().BuildConfigs(ns).List(opts)
		},
		WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
			return buildclient.BuildV1().BuildConfigs(ns).Watch(opts)
		},
	}
}

func createBuildClient(apiserver string, kubeconfig string) (*buildclient.Clientset, error) {
	config, err := clientcmd.BuildConfigFromFlags(apiserver, kubeconfig)
	if err != nil {
		return nil, err
	}

	config.UserAgent = version.GetVersion().String()
	config.AcceptContentTypes = "application/vnd.kubernetes.protobuf,application/json"
	config.ContentType = "application/vnd.kubernetes.protobuf"

	client, err := buildclient.NewForConfig(config)
	return client, err

}
