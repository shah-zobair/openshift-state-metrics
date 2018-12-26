package options

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kube-state-metrics/pkg/options"
)

var (
	DefaultNamespaces = options.NamespaceList{metav1.NamespaceAll}
	DefaultCollectors = options.CollectorSet{
		"deploymentConfigs": struct{}{},
	}
)
