/*
Copyright 2018 The Kubernetes Authors All rights reserved.

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

//
package collectors

import (
	"sort"
	"strings"

	"k8s.io/kube-state-metrics/pkg/collectors"
	"k8s.io/kube-state-metrics/pkg/metrics"
	"k8s.io/kube-state-metrics/pkg/metrics_store"
	"k8s.io/kube-state-metrics/pkg/options"

	"k8s.io/client-go/tools/cache"

	"github.com/golang/glog"
	appsv1 "github.com/openshift/api/apps/v1"
	buildv1 "github.com/openshift/api/build/v1"
	quotav1 "github.com/openshift/api/quota/v1"
	"golang.org/x/net/context"
)

type whiteBlackLister interface {
	IsIncluded(string) bool
	IsExcluded(string) bool
}

// Builder helps to build collectors. It follows the builder pattern
// (https://en.wikipedia.org/wiki/Builder_pattern).
type Builder struct {
	apiserver         string
	kubeconfig        string
	namespaces        options.NamespaceList
	ctx               context.Context
	enabledCollectors []string
	whiteBlackList    whiteBlackLister
}

// NewBuilder returns a new builder.
func NewBuilder(
	ctx context.Context,
) *Builder {
	return &Builder{
		ctx: ctx,
	}
}

func (b *Builder) WithApiserver(apiserver string) *Builder {
	b.apiserver = apiserver
	return b
}

func (b *Builder) WithKubeConfig(kubeconfig string) *Builder {
	b.kubeconfig = kubeconfig
	return b
}

// WithEnabledCollectors sets the enabledCollectors property of a Builder.
func (b *Builder) WithEnabledCollectors(c []string) *Builder {
	copy := []string{}
	for _, s := range c {
		copy = append(copy, s)
	}

	sort.Strings(copy)

	b.enabledCollectors = copy
	return b
}

// WithNamespaces sets the namespaces property of a Builder.
func (b *Builder) WithNamespaces(n options.NamespaceList) *Builder {
	b.namespaces = n
	return b
}

// WithWhiteBlackList configures the white or blacklisted metrics to be exposed
// by the collectors build by the Builder
func (b *Builder) WithWhiteBlackList(l whiteBlackLister) *Builder {
	b.whiteBlackList = l
	return b
}

// Build initializes and registers all enabled collectors.
func (b *Builder) Build() []*collectors.Collector {
	if b.whiteBlackList == nil {
		panic("whiteBlackList should not be nil")
	}

	collectors := []*collectors.Collector{}
	activeCollectorNames := []string{}

	for _, c := range b.enabledCollectors {
		constructor, ok := availableCollectors[c]
		if !ok {
			glog.Fatalf("collector %s is not correct", c)
		}

		collector := constructor(b)
		activeCollectorNames = append(activeCollectorNames, c)
		collectors = append(collectors, collector)

	}

	glog.Infof("Active collectors: %s", strings.Join(activeCollectorNames, ","))

	return collectors
}

var availableCollectors = map[string]func(f *Builder) *collectors.Collector{
	"deploymentConfigs":     func(b *Builder) *collectors.Collector { return b.buildDeploymentCollector() },
	"buildconfigs":          func(b *Builder) *collectors.Collector { return b.buildBuildConfigCollector() },
	"builds":                func(b *Builder) *collectors.Collector { return b.buildBuildCollector() },
	"clusterresourcequotas": func(b *Builder) *collectors.Collector { return b.buildQuotaCollector() },
}

func extractMetricFamilyHeaders(families []metrics.FamilyGenerator) []string {
	headers := make([]string, len(families))

	for i, f := range families {
		header := strings.Builder{}

		header.WriteString("# HELP ")
		header.WriteString(f.Name)
		header.WriteByte(' ')
		header.WriteString(f.Help)
		header.WriteByte('\n')
		header.WriteString("# TYPE ")
		header.WriteString(f.Name)
		header.WriteByte(' ')
		header.WriteString(string(f.Type))

		headers[i] = header.String()
	}

	return headers
}

// composeMetricGenFuncs takes a slice of metric families and returns a function
// that composes their metric generation functions into a single one.
func composeMetricGenFuncs(families []metrics.FamilyGenerator) func(obj interface{}) []metricsstore.FamilyStringer {
	funcs := []func(obj interface{}) metrics.Family{}

	for _, f := range families {
		funcs = append(funcs, f.GenerateFunc)
	}

	return func(obj interface{}) []metricsstore.FamilyStringer {
		families := make([]metricsstore.FamilyStringer, len(funcs))

		for i, f := range funcs {
			families[i] = f(obj)
		}

		return families
	}
}

func (b *Builder) buildDeploymentCollector() *collectors.Collector {
	filteredMetricFamilies := filterMetricFamilies(b.whiteBlackList, deploymentMetricFamilies)
	composedMetricGenFuncs := composeMetricGenFuncs(filteredMetricFamilies)

	familyHeaders := extractMetricFamilyHeaders(filteredMetricFamilies)

	store := metricsstore.NewMetricsStore(
		familyHeaders,
		composedMetricGenFuncs,
	)
	reflectorPerNamespace(b.ctx, &appsv1.DeploymentConfig{}, store,
		b.apiserver, b.kubeconfig, b.namespaces, createDeploymentListWatch)

	return collectors.NewCollector(store)
}

func (b *Builder) buildBuildConfigCollector() *collectors.Collector {
	filteredMetricFamilies := filterMetricFamilies(b.whiteBlackList, buildconfigMetricFamilies)
	composedMetricGenFuncs := composeMetricGenFuncs(filteredMetricFamilies)

	familyHeaders := extractMetricFamilyHeaders(filteredMetricFamilies)

	store := metricsstore.NewMetricsStore(
		familyHeaders,
		composedMetricGenFuncs,
	)
	reflectorPerNamespace(b.ctx, &buildv1.BuildConfig{}, store,
		b.apiserver, b.kubeconfig, b.namespaces, createBuildConfigListWatch)

	return collectors.NewCollector(store)
}

func (b *Builder) buildBuildCollector() *collectors.Collector {
	filteredMetricFamilies := filterMetricFamilies(b.whiteBlackList, buildMetricFamilies)
	composedMetricGenFuncs := composeMetricGenFuncs(filteredMetricFamilies)

	familyHeaders := extractMetricFamilyHeaders(filteredMetricFamilies)

	store := metricsstore.NewMetricsStore(
		familyHeaders,
		composedMetricGenFuncs,
	)
	reflectorPerNamespace(b.ctx, &buildv1.Build{}, store,
		b.apiserver, b.kubeconfig, b.namespaces, createBuildListWatch)

	return collectors.NewCollector(store)
}

func (b *Builder) buildQuotaCollector() *collectors.Collector {
	filteredMetricFamilies := filterMetricFamilies(b.whiteBlackList, quotaMetricFamilies)
	composedMetricGenFuncs := composeMetricGenFuncs(filteredMetricFamilies)

	familyHeaders := extractMetricFamilyHeaders(filteredMetricFamilies)

	store := metricsstore.NewMetricsStore(
		familyHeaders,
		composedMetricGenFuncs,
	)
	reflectorPerNamespace(b.ctx, &quotav1.ClusterResourceQuota{}, store,
		b.apiserver, b.kubeconfig, b.namespaces, createClusterResourceQuotaListWatch)

	return collectors.NewCollector(store)
}

// filterMetricFamilies takes a white- and a blacklist and a slice of metric
// families and returns a filtered slice.
func filterMetricFamilies(l whiteBlackLister, families []metrics.FamilyGenerator) []metrics.FamilyGenerator {
	filtered := []metrics.FamilyGenerator{}

	for _, f := range families {
		if l.IsIncluded(f.Name) {
			filtered = append(filtered, f)
		}
	}

	return filtered
}

// reflectorPerNamespace creates a Kubernetes client-go reflector with the given
// listWatchFunc for each given namespace and registers it with the given store.
func reflectorPerNamespace(
	ctx context.Context,
	expectedType interface{},
	store cache.Store,
	apiserver string,
	kubeconfig string,
	namespaces []string,
	listWatchFunc func(apiserver string, kubeconfig string, ns string) cache.ListWatch,
) {
	for _, ns := range namespaces {
		lw := listWatchFunc(apiserver, kubeconfig, ns)
		reflector := cache.NewReflector(&lw, expectedType, store, 0)
		go reflector.Run(ctx.Done())
	}
}
