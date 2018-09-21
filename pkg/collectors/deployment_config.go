/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/golang/glog"
	"github.com/openshift/api/apps/v1"
	appsclient "github.com/openshift/client-go/apps/clientset/versioned"
	informers "github.com/openshift/client-go/apps/informers/externalversions"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/wanghaoran1988/openshift-state-metrics/pkg/options"
	"github.com/wanghaoran1988/openshift-state-metrics/pkg/version"
	"golang.org/x/net/context"
)

var (
	descDeploymentConfigLabelsName          = "openshift_deploymentconfig_labels"
	descDeploymentConfigLabelsHelp          = "openshift labels converted to Prometheus labels."
	descDeploymentConfigLabelsDefaultLabels = []string{"namespace", "deploymentconfig"}

	descDeploymentConfigCreated = prometheus.NewDesc(
		"openshift_deploymentconfig_created",
		"Unix creation timestamp",
		descDeploymentConfigLabelsDefaultLabels,
		nil,
	)

	descDeploymentConfigStatusReplicas = prometheus.NewDesc(
		"openshift_deploymentconfig_status_replicas",
		"The number of replicas per deploymentconfig.",
		descDeploymentConfigLabelsDefaultLabels,
		nil,
	)
	descDeploymentConfigStatusReplicasAvailable = prometheus.NewDesc(
		"openshift_deploymentconfig_status_replicas_available",
		"The number of available replicas per deploymentconfig.",
		descDeploymentConfigLabelsDefaultLabels,
		nil,
	)
	descDeploymentConfigStatusReplicasUnavailable = prometheus.NewDesc(
		"openshift_deploymentconfig_status_replicas_unavailable",
		"The number of unavailable replicas per deploymentconfig.",
		descDeploymentConfigLabelsDefaultLabels,
		nil,
	)
	descDeploymentConfigStatusReplicasUpdated = prometheus.NewDesc(
		"openshift_deploymentconfig_status_replicas_updated",
		"The number of updated replicas per deploymentconfig.",
		descDeploymentConfigLabelsDefaultLabels,
		nil,
	)

	descDeploymentConfigStatusObservedGeneration = prometheus.NewDesc(
		"openshift_deploymentconfig_status_observed_generation",
		"The generation observed by the deploymentconfig controller.",
		descDeploymentConfigLabelsDefaultLabels,
		nil,
	)

	descDeploymentConfigSpecReplicas = prometheus.NewDesc(
		"openshift_deploymentconfig_spec_replicas",
		"Number of desired pods for a deploymentconfig.",
		descDeploymentConfigLabelsDefaultLabels,
		nil,
	)

	descDeploymentConfigSpecPaused = prometheus.NewDesc(
		"openshift_deploymentconfig_spec_paused",
		"Whether the deployment is paused and will not be processed by the deploymentconfig controller.",
		descDeploymentConfigLabelsDefaultLabels,
		nil,
	)

	descDeploymentConfigStrategyRollingUpdateMaxUnavailable = prometheus.NewDesc(
		"openshift_deploymentconfig_spec_strategy_rollingupdate_max_unavailable",
		"Maximum number of unavailable replicas during a rolling update of a deploymentconfig.",
		descDeploymentConfigLabelsDefaultLabels,
		nil,
	)

	descDeploymentConfigStrategyRollingUpdateMaxSurge = prometheus.NewDesc(
		"openshift_deploymentconfig_spec_strategy_rollingupdate_max_surge",
		"Maximum number of replicas that can be scheduled above the desired number of replicas during a rolling update of a deploymentconfig.",
		descDeploymentConfigLabelsDefaultLabels,
		nil,
	)

	descDeploymentConfigMetadataGeneration = prometheus.NewDesc(
		"openshift_deploymentconfig_metadata_generation",
		"Sequence number representing a specific generation of the desired state.",
		descDeploymentConfigLabelsDefaultLabels,
		nil,
	)

	descDeploymentConfigLabels = prometheus.NewDesc(
		descDeploymentConfigLabelsName,
		descDeploymentConfigLabelsHelp,
		descDeploymentConfigLabelsDefaultLabels, nil,
	)
)

type DeploymentLister func() ([]v1.DeploymentConfig, error)

func (l DeploymentLister) List() ([]v1.DeploymentConfig, error) {
	return l()
}

func createAppsClient(apiserver string, kubeconfig string) (*appsclient.Clientset,error){
	config, err := clientcmd.BuildConfigFromFlags(apiserver, kubeconfig)
	if err != nil {
		return nil, err
	}

	config.UserAgent = version.GetVersion().String()
	config.AcceptContentTypes = "application/vnd.kubernetes.protobuf,application/json"
	config.ContentType = "application/vnd.kubernetes.protobuf"

	client, err := appsclient.NewForConfig(config)
	return client,err

}

func RegisterDeploymentConfigCollector(registry prometheus.Registerer, opts *options.Options) {
	glog.Info("register deployment config collector")
	appsclient, err := createAppsClient(opts.Apiserver,opts.Kubeconfig)

	if err!=nil{
		glog.Fatalf("failed :%v", err)
	}

	infos := []informers.SharedInformerFactory{}
	for _, ns := range opts.Namespaces{
		infos = append(infos, informers.NewSharedInformerFactoryWithOptions(appsclient,0, informers.WithNamespace(ns)))
	}
	infs := SharedInformerList{}
	for _, f := range infos {
		infs = append(infs, f.Apps().V1().DeploymentConfigs().Informer().(cache.SharedInformer))
	}

	dplLister := DeploymentLister(func() (deployments []v1.DeploymentConfig, err error) {
		for _, dinf := range infs {
			for _, c := range dinf.GetStore().List() {
				deployments = append(deployments, *(c.(*v1.DeploymentConfig)))
			}
		}
		return deployments, nil
	})

	registry.MustRegister(&deploymentCollector{store: dplLister, opts: opts})
	infs.Run(context.Background().Done())
}

type deploymentStore interface {
	List() (deployments []v1.DeploymentConfig, err error)
}

// deploymentCollector collects metrics about all deployments in the cluster.
type deploymentCollector struct {
	store deploymentStore
	opts  *options.Options
}

// Describe implements the prometheus.Collector interface.
func (dc *deploymentCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- descDeploymentConfigCreated
	ch <- descDeploymentConfigStatusReplicas
	ch <- descDeploymentConfigStatusReplicasAvailable
	ch <- descDeploymentConfigStatusReplicasUnavailable
	ch <- descDeploymentConfigStatusReplicasUpdated
	ch <- descDeploymentConfigStatusObservedGeneration
	ch <- descDeploymentConfigSpecPaused
	ch <- descDeploymentConfigStrategyRollingUpdateMaxUnavailable
	ch <- descDeploymentConfigStrategyRollingUpdateMaxSurge
	ch <- descDeploymentConfigSpecReplicas
	ch <- descDeploymentConfigMetadataGeneration
	ch <- descDeploymentConfigLabels
}

// Collect implements the prometheus.Collector interface.
func (dc *deploymentCollector) Collect(ch chan<- prometheus.Metric) {
	ds, err := dc.store.List()
	if err != nil {
		ScrapeErrorTotalMetric.With(prometheus.Labels{"resource": "deploymentconfig"}).Inc()
		glog.Errorf("listing deployments failed: %s", err)
		return
	}
	ScrapeErrorTotalMetric.With(prometheus.Labels{"resource": "deploymentconfig"}).Add(0)

	ResourcesPerScrapeMetric.With(prometheus.Labels{"resource": "deploymentconfig"}).Observe(float64(len(ds)))
	for _, d := range ds {
		dc.collectDeploymentConfig(ch, d)
	}

	glog.V(4).Infof("collected %d deploymentconfigs", len(ds))
}

func deploymentLabelsDesc(labelKeys []string) *prometheus.Desc {
	return prometheus.NewDesc(
		descDeploymentConfigLabelsName,
		descDeploymentConfigLabelsHelp,
		append(descDeploymentConfigLabelsDefaultLabels, labelKeys...),
		nil,
	)
}

func (dc *deploymentCollector) collectDeploymentConfig(ch chan<- prometheus.Metric, d v1.DeploymentConfig) {
	addGauge := func(desc *prometheus.Desc, v float64, lv ...string) {
		lv = append([]string{d.Namespace, d.Name}, lv...)
		ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, v, lv...)
	}
	labelKeys, labelValues := kubeLabelsToPrometheusLabels(d.Labels)
	addGauge(deploymentLabelsDesc(labelKeys), 1, labelValues...)
	if !d.CreationTimestamp.IsZero() {
		addGauge(descDeploymentConfigCreated, float64(d.CreationTimestamp.Unix()))
	}
	addGauge(descDeploymentConfigStatusReplicas, float64(d.Status.Replicas))
	addGauge(descDeploymentConfigStatusReplicasAvailable, float64(d.Status.AvailableReplicas))
	addGauge(descDeploymentConfigStatusReplicasUnavailable, float64(d.Status.UnavailableReplicas))
	addGauge(descDeploymentConfigStatusReplicasUpdated, float64(d.Status.UpdatedReplicas))
	addGauge(descDeploymentConfigStatusObservedGeneration, float64(d.Status.ObservedGeneration))
	addGauge(descDeploymentConfigSpecPaused, boolFloat64(d.Spec.Paused))
	addGauge(descDeploymentConfigSpecReplicas, float64(d.Spec.Replicas))
	addGauge(descDeploymentConfigMetadataGeneration, float64(d.ObjectMeta.Generation))

	if d.Spec.Strategy.RollingParams == nil {
		return
	}

	maxUnavailable, err := intstr.GetValueFromIntOrPercent(d.Spec.Strategy.RollingParams.MaxUnavailable, int(d.Spec.Replicas), true)
	if err != nil {
		glog.Errorf("Error converting RollingUpdate MaxUnavailable to int: %s", err)
	} else {
		addGauge(descDeploymentConfigStrategyRollingUpdateMaxUnavailable, float64(maxUnavailable))
	}

	maxSurge, err := intstr.GetValueFromIntOrPercent(d.Spec.Strategy.RollingParams.MaxSurge, int(d.Spec.Replicas), true)
	if err != nil {
		glog.Errorf("Error converting RollingUpdate MaxSurge to int: %s", err)
	} else {
		addGauge(descDeploymentConfigStrategyRollingUpdateMaxSurge, float64(maxSurge))
	}

}
