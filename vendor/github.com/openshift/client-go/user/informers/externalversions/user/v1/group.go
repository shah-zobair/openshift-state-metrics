// Code generated by informer-gen. DO NOT EDIT.

package v1

import (
	time "time"

	user_v1 "github.com/openshift/api/user/v1"
	versioned "github.com/openshift/client-go/user/clientset/versioned"
	internalinterfaces "github.com/openshift/client-go/user/informers/externalversions/internalinterfaces"
	v1 "github.com/openshift/client-go/user/listers/user/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// GroupInformer provides access to a shared informer and lister for
// Groups.
type GroupInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1.GroupLister
}

type groupInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
}

// NewGroupInformer constructs a new informer for Group type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewGroupInformer(client versioned.Interface, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredGroupInformer(client, resyncPeriod, indexers, nil)
}

// NewFilteredGroupInformer constructs a new informer for Group type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredGroupInformer(client versioned.Interface, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options meta_v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.UserV1().Groups().List(options)
			},
			WatchFunc: func(options meta_v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.UserV1().Groups().Watch(options)
			},
		},
		&user_v1.Group{},
		resyncPeriod,
		indexers,
	)
}

func (f *groupInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredGroupInformer(client, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *groupInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&user_v1.Group{}, f.defaultInformer)
}

func (f *groupInformer) Lister() v1.GroupLister {
	return v1.NewGroupLister(f.Informer().GetIndexer())
}
