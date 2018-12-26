package collectors

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/kube-state-metrics/pkg/metrics"

	"github.com/golang/glog"

	"github.com/openshift/api/build/v1"
)

var (
	descBuildLabelsName          = "openshift_build_labels"
	descBuildLabelsHelp          = "Kubernetes labels converted to Prometheus labels."
	descBuildLabelsDefaultLabels = []string{"namespace", "build"}

	buildMetricFamilies = []metrics.FamilyGenerator{
		metrics.FamilyGenerator{
			Name: "openshift_build_created",
			Type: metrics.MetricTypeGauge,
			Help: "Unix creation timestamp",
			GenerateFunc: wrapBuildFunc(func(b *v1.Build) metrics.Family {
				f := metrics.Family{}

				if !b.CreationTimestamp.IsZero() {
					f = append(f, &metrics.Metric{
						Name:  "openshift_build_created",
						Value: float64(b.CreationTimestamp.Unix()),
					})
				}

				return f
			}),
		},
		metrics.FamilyGenerator{
			Name: "openshift_build_metadata_generation",
			Type: metrics.MetricTypeGauge,
			Help: "Sequence number representing a specific generation of the desired state.",
			GenerateFunc: wrapBuildFunc(func(b *v1.Build) metrics.Family {
				return metrics.Family{&metrics.Metric{
					Name:  "openshift_build_metadata_generation",
					Value: float64(b.ObjectMeta.Generation),
				}}
			}),
		},
		metrics.FamilyGenerator{
			Name: descBuildLabelsName,
			Type: metrics.MetricTypeGauge,
			Help: descBuildLabelsHelp,
			GenerateFunc: wrapBuildFunc(func(b *v1.Build) metrics.Family {
				labelKeys, labelValues := kubeLabelsToPrometheusLabels(b.Labels)
				return metrics.Family{&metrics.Metric{
					Name:        descBuildLabelsName,
					LabelKeys:   labelKeys,
					LabelValues: labelValues,
					Value:       1,
				}}
			}),
		},
		metrics.FamilyGenerator{
			Name: "openshift_build_status_phase",
			Type: metrics.MetricTypeGauge,
			Help: "The build phase.",
			GenerateFunc: wrapBuildFunc(func(b *v1.Build) metrics.Family {
				f := metrics.Family{}
				ms := addBuildPahseMetrics(b.Status.Phase)
				for _, m := range ms {
					metric := m
					metric.Name = "openshift_build_status_phase"
					metric.LabelKeys = []string{"build_phase"}
					f = append(f, metric)
				}
				return f
			}),
		},
		metrics.FamilyGenerator{
			Name: "openshift_build_started",
			Type: metrics.MetricTypeGauge,
			Help: "Started time of the build",
			GenerateFunc: wrapBuildFunc(func(b *v1.Build) metrics.Family {
				f := metrics.Family{}

				if !b.CreationTimestamp.IsZero() {
					f = append(f, &metrics.Metric{
						Name:  "openshift_build_started",
						Value: float64(b.Status.StartTimestamp.Unix()),
					})
				}

				return f
			}),
		},
		metrics.FamilyGenerator{
			Name: "openshift_build_complete",
			Type: metrics.MetricTypeGauge,
			Help: "Started time of the build",
			GenerateFunc: wrapBuildFunc(func(b *v1.Build) metrics.Family {
				f := metrics.Family{}

				if !b.CreationTimestamp.IsZero() {
					f = append(f, &metrics.Metric{
						Name:  "openshift_build_complete",
						Value: float64(b.Status.CompletionTimestamp.Unix()),
					})
				}

				return f
			}),
		},
		metrics.FamilyGenerator{
			Name: "openshift_build_duration",
			Type: metrics.MetricTypeGauge,
			Help: "Started time of the build",
			GenerateFunc: wrapBuildFunc(func(b *v1.Build) metrics.Family {
				f := metrics.Family{}

				if !b.CreationTimestamp.IsZero() {
					f = append(f, &metrics.Metric{
						Name:  "openshift_build_duration",
						Value: float64(b.Status.Duration),
					})
				}

				return f
			}),
		},
	}
)

func wrapBuildFunc(f func(config *v1.Build) metrics.Family) func(interface{}) metrics.Family {
	return func(obj interface{}) metrics.Family {
		build := obj.(*v1.Build)

		metricFamily := f(build)

		for _, m := range metricFamily {
			m.LabelKeys = append(descBuildLabelsDefaultLabels, m.LabelKeys...)
			m.LabelValues = append([]string{build.Namespace, build.Name}, m.LabelValues...)
		}

		return metricFamily
	}
}

func createBuildListWatch(apiserver string, kubeconfig string, ns string) cache.ListWatch {
	buildclient, err := createBuildClient(apiserver, kubeconfig)
	if err != nil {
		glog.Fatalf("cannot create build client: %v", err)
	}
	return cache.ListWatch{
		ListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {
			return buildclient.BuildV1().Builds(ns).List(opts)
		},
		WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
			return buildclient.BuildV1().Builds(ns).Watch(opts)
		},
	}
}

// addConditionMetrics generates one metric for each possible node condition
// status. For this function to work properly, the last label in the metric
// description must be the condition.
func addBuildPahseMetrics(cs v1.BuildPhase) []*metrics.Metric {
	return []*metrics.Metric{
		&metrics.Metric{
			LabelValues: []string{"complete"},
			Value:       boolFloat64(cs == v1.BuildPhaseComplete),
		},
		&metrics.Metric{
			LabelValues: []string{"cancelled"},
			Value:       boolFloat64(cs == v1.BuildPhaseCancelled),
		},
		&metrics.Metric{
			LabelValues: []string{"new"},
			Value:       boolFloat64(cs == v1.BuildPhaseNew),
		},
		&metrics.Metric{
			LabelValues: []string{"pending"},
			Value:       boolFloat64(cs == v1.BuildPhasePending),
		},
		&metrics.Metric{
			LabelValues: []string{"running"},
			Value:       boolFloat64(cs == v1.BuildPhaseRunning),
		},
		&metrics.Metric{
			LabelValues: []string{"failed"},
			Value:       boolFloat64(cs == v1.BuildPhaseFailed),
		},
		&metrics.Metric{
			LabelValues: []string{"error"},
			Value:       boolFloat64(cs == v1.BuildPhaseError),
		},
	}
}
