package collectors

import (
	"k8s.io/kube-state-metrics/pkg/metrics"
	"k8s.io/kube-state-metrics/pkg/version"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/golang/glog"

	"github.com/openshift/api/apps/v1"
	appsclient "github.com/openshift/client-go/apps/clientset/versioned"
)

var (
	descDeploymentLabelsName          = "openshift_deploymentconfig_labels"
	descDeploymentLabelsHelp          = "Kubernetes labels converted to Prometheus labels."
	descDeploymentLabelsDefaultLabels = []string{"namespace", "deploymentconfig"}

	deploymentMetricFamilies = []metrics.FamilyGenerator{
		metrics.FamilyGenerator{
			Name: "openshift_deploymentconfig_created",
			Type: metrics.MetricTypeGauge,
			Help: "Unix creation timestamp",
			GenerateFunc: wrapDeploymentFunc(func(d *v1.DeploymentConfig) metrics.Family {
				f := metrics.Family{}

				if !d.CreationTimestamp.IsZero() {
					f = append(f, &metrics.Metric{
						Name:  "openshift_deploymentconfig_created",
						Value: float64(d.CreationTimestamp.Unix()),
					})
				}

				return f
			}),
		},
		metrics.FamilyGenerator{
			Name: "openshift_deploymentconfig_status_replicas",
			Type: metrics.MetricTypeGauge,
			Help: "The number of replicas per deployment.",
			GenerateFunc: wrapDeploymentFunc(func(d *v1.DeploymentConfig) metrics.Family {
				return metrics.Family{&metrics.Metric{
					Name:  "openshift_deploymentconfig_status_replicas",
					Value: float64(d.Status.Replicas),
				}}
			}),
		},
		metrics.FamilyGenerator{
			Name: "openshift_deploymentconfig_status_replicas_available",
			Type: metrics.MetricTypeGauge,
			Help: "The number of available replicas per deployment.",
			GenerateFunc: wrapDeploymentFunc(func(d *v1.DeploymentConfig) metrics.Family {
				return metrics.Family{&metrics.Metric{
					Name:  "openshift_deploymentconfig_status_replicas_available",
					Value: float64(d.Status.AvailableReplicas),
				}}
			}),
		},
		metrics.FamilyGenerator{
			Name: "openshift_deploymentconfig_status_replicas_unavailable",
			Type: metrics.MetricTypeGauge,
			Help: "The number of unavailable replicas per deployment.",
			GenerateFunc: wrapDeploymentFunc(func(d *v1.DeploymentConfig) metrics.Family {
				return metrics.Family{&metrics.Metric{
					Name:  "openshift_deploymentconfig_status_replicas_unavailable",
					Value: float64(d.Status.UnavailableReplicas),
				}}
			}),
		},
		metrics.FamilyGenerator{
			Name: "openshift_deploymentconfig_status_replicas_updated",
			Type: metrics.MetricTypeGauge,
			Help: "The number of updated replicas per deployment.",
			GenerateFunc: wrapDeploymentFunc(func(d *v1.DeploymentConfig) metrics.Family {
				return metrics.Family{&metrics.Metric{
					Name:  "openshift_deploymentconfig_status_replicas_updated",
					Value: float64(d.Status.UpdatedReplicas),
				}}
			}),
		},
		metrics.FamilyGenerator{
			Name: "openshift_deploymentconfig_status_observed_generation",
			Type: metrics.MetricTypeGauge,
			Help: "The generation observed by the deployment controller.",
			GenerateFunc: wrapDeploymentFunc(func(d *v1.DeploymentConfig) metrics.Family {
				return metrics.Family{&metrics.Metric{
					Name:  "openshift_deploymentconfig_status_observed_generation",
					Value: float64(d.Status.ObservedGeneration),
				}}
			}),
		},
		metrics.FamilyGenerator{
			Name: "openshift_deploymentconfig_spec_replicas",
			Type: metrics.MetricTypeGauge,
			Help: "Number of desired pods for a deployment.",
			GenerateFunc: wrapDeploymentFunc(func(d *v1.DeploymentConfig) metrics.Family {
				return metrics.Family{&metrics.Metric{
					Name:  "openshift_deploymentconfig_spec_replicas",
					Value: float64(d.Spec.Replicas),
				}}
			}),
		},
		metrics.FamilyGenerator{
			Name: "openshift_deploymentconfig_spec_paused",
			Type: metrics.MetricTypeGauge,
			Help: "Whether the deployment is paused and will not be processed by the deployment controller.",
			GenerateFunc: wrapDeploymentFunc(func(d *v1.DeploymentConfig) metrics.Family {
				return metrics.Family{&metrics.Metric{
					Name:  "openshift_deploymentconfig_spec_paused",
					Value: boolFloat64(d.Spec.Paused),
				}}
			}),
		},
		metrics.FamilyGenerator{
			Name: "openshift_deploymentconfig_spec_strategy_rollingupdate_max_unavailable",
			Type: metrics.MetricTypeGauge,
			Help: "Maximum number of unavailable replicas during a rolling update of a deployment.",
			GenerateFunc: wrapDeploymentFunc(func(d *v1.DeploymentConfig) metrics.Family {
				if d.Spec.Strategy.RollingParams == nil {
					return metrics.Family{}
				}

				maxUnavailable, err := intstr.GetValueFromIntOrPercent(d.Spec.Strategy.RollingParams.MaxUnavailable, int(d.Spec.Replicas), true)
				if err != nil {
					panic(err)
				}

				return metrics.Family{&metrics.Metric{
					Name:  "openshift_deploymentconfig_spec_strategy_rollingupdate_max_unavailable",
					Value: float64(maxUnavailable),
				}}
			}),
		},
		metrics.FamilyGenerator{
			Name: "openshift_deploymentconfig_spec_strategy_rollingupdate_max_surge",
			Type: metrics.MetricTypeGauge,
			Help: "Maximum number of replicas that can be scheduled above the desired number of replicas during a rolling update of a deployment.",
			GenerateFunc: wrapDeploymentFunc(func(d *v1.DeploymentConfig) metrics.Family {
				if d.Spec.Strategy.RollingParams == nil {
					return metrics.Family{}
				}

				maxSurge, err := intstr.GetValueFromIntOrPercent(d.Spec.Strategy.RollingParams.MaxSurge, int(d.Spec.Replicas), true)
				if err != nil {
					panic(err)
				}

				return metrics.Family{&metrics.Metric{
					Name:  "openshift_deploymentconfig_spec_strategy_rollingupdate_max_surge",
					Value: float64(maxSurge),
				}}
			}),
		},
		metrics.FamilyGenerator{
			Name: "openshift_deploymentconfig_metadata_generation",
			Type: metrics.MetricTypeGauge,
			Help: "Sequence number representing a specific generation of the desired state.",
			GenerateFunc: wrapDeploymentFunc(func(d *v1.DeploymentConfig) metrics.Family {
				return metrics.Family{&metrics.Metric{
					Name:  "openshift_deploymentconfig_metadata_generation",
					Value: float64(d.ObjectMeta.Generation),
				}}
			}),
		},
		metrics.FamilyGenerator{
			Name: descDeploymentLabelsName,
			Type: metrics.MetricTypeGauge,
			Help: descDeploymentLabelsHelp,
			GenerateFunc: wrapDeploymentFunc(func(d *v1.DeploymentConfig) metrics.Family {
				labelKeys, labelValues := kubeLabelsToPrometheusLabels(d.Labels)
				return metrics.Family{&metrics.Metric{
					Name:        descDeploymentLabelsName,
					LabelKeys:   labelKeys,
					LabelValues: labelValues,
					Value:       1,
				}}
			}),
		},
	}
)

func wrapDeploymentFunc(f func(*v1.DeploymentConfig) metrics.Family) func(interface{}) metrics.Family {
	return func(obj interface{}) metrics.Family {
		deployment := obj.(*v1.DeploymentConfig)

		metricFamily := f(deployment)

		for _, m := range metricFamily {
			m.LabelKeys = append(descDeploymentLabelsDefaultLabels, m.LabelKeys...)
			m.LabelValues = append([]string{deployment.Namespace, deployment.Name}, m.LabelValues...)
		}

		return metricFamily
	}
}

func createDeploymentListWatch(apiserver string, kubeconfig string, ns string) cache.ListWatch {
	appsclient, err := createAppsClient(apiserver, kubeconfig)
	if err != nil {
		glog.Fatalf("cannot create deploymentconfig client:", err)
	}
	return cache.ListWatch{
		ListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {
			return appsclient.AppsV1().DeploymentConfigs(ns).List(opts)
		},
		WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
			return appsclient.AppsV1().DeploymentConfigs(ns).Watch(opts)
		},
	}
}

func createAppsClient(apiserver string, kubeconfig string) (*appsclient.Clientset, error) {
	config, err := clientcmd.BuildConfigFromFlags(apiserver, kubeconfig)
	if err != nil {
		return nil, err
	}

	config.UserAgent = version.GetVersion().String()
	config.AcceptContentTypes = "application/vnd.kubernetes.protobuf,application/json"
	config.ContentType = "application/vnd.kubernetes.protobuf"

	client, err := appsclient.NewForConfig(config)
	return client, err

}
