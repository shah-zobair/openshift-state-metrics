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

package main

import (
	"fmt"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"log"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"strconv"
	"strings"

	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/util/proc"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	ocollectors "github.com/wanghaoran1988/openshift-state-metrics/pkg/collectors"
	"github.com/wanghaoran1988/openshift-state-metrics/pkg/metrics"
	"github.com/wanghaoran1988/openshift-state-metrics/pkg/options"
	"github.com/wanghaoran1988/openshift-state-metrics/pkg/version"
)

const (
	metricsPath = "/metrics"
	healthzPath = "/healthz"
)

// promLogger implements promhttp.Logger
type promLogger struct{}

func (pl promLogger) Println(v ...interface{}) {
	glog.Error(v)
}

func main() {
	opts := options.NewOptions()
	opts.AddFlags()

	err := opts.Parse()
	if err != nil {
		glog.Fatalf("Error: %s", err)
	}

	if opts.Version {
		fmt.Printf("%#v\n", version.GetVersion())
		os.Exit(0)
	}

	if opts.Help {
		opts.Usage()
		os.Exit(0)
	}

	var collectors options.CollectorSet
	if len(opts.Collectors) == 0 {
		glog.Info("Using default collectors")
		collectors = options.DefaultCollectors
	} else {
		collectors = opts.Collectors
	}

	var namespaces options.NamespaceList
	if len(opts.Namespaces) == 0 {
		namespaces = options.DefaultNamespaces
		opts.Namespaces = options.DefaultNamespaces
	} else {
		namespaces = opts.Namespaces
	}

	if namespaces.IsAllNamespaces() {
		glog.Info("Using all namespace")
	} else {
		glog.Infof("Using %s namespaces", namespaces)
	}

	if opts.MetricWhitelist.IsEmpty() && opts.MetricBlacklist.IsEmpty() {
		glog.Info("No metric whitelist or blacklist set. No filtering of metrics will be done.")
	}
	if !opts.MetricWhitelist.IsEmpty() && !opts.MetricBlacklist.IsEmpty() {
		glog.Fatal("Whitelist and blacklist are both set. They are mutually exclusive, only one of them can be set.")
	}
	if !opts.MetricWhitelist.IsEmpty() {
		glog.Infof("A metric whitelist has been configured. Only the following metrics will be exposed: %s.", opts.MetricWhitelist.String())
	}
	if !opts.MetricBlacklist.IsEmpty() {
		glog.Infof("A metric blacklist has been configured. The following metrics will not be exposed: %s.", opts.MetricBlacklist.String())
	}

	proc.StartReaper()

	ksmMetricsRegistry := prometheus.NewRegistry()
	ksmMetricsRegistry.Register(ocollectors.ResourcesPerScrapeMetric)
	ksmMetricsRegistry.Register(ocollectors.ScrapeErrorTotalMetric)
	ksmMetricsRegistry.Register(prometheus.NewProcessCollector(os.Getpid(),""))
	ksmMetricsRegistry.Register(prometheus.NewGoCollector())
	go telemetryServer(ksmMetricsRegistry, opts.TelemetryHost, opts.TelemetryPort)

	registry := prometheus.NewRegistry()
	registerCollectors(registry, collectors, opts)
	metricsServer(metrics.FilteredGatherer(registry, opts.MetricWhitelist, opts.MetricBlacklist), opts.Host, opts.Port)
}

func telemetryServer(registry prometheus.Gatherer, host string, port int) {
	// Address to listen on for web interface and telemetry
	listenAddress := net.JoinHostPort(host, strconv.Itoa(port))

	glog.Infof("Starting openshift-state-metrics self metrics server: %s", listenAddress)

	mux := http.NewServeMux()

	// Add metricsPath
	mux.Handle(metricsPath, promhttp.HandlerFor(registry, promhttp.HandlerOpts{ErrorLog: promLogger{}}))
	// Add index
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
             <head><title>OpenShift-State-Metrics Metrics Server</title></head>
             <body>
             <h1>OpenShift-State-Metrics Metrics</h1>
			 <ul>
             <li><a href='` + metricsPath + `'>metrics</a></li>
			 </ul>
             </body>
             </html>`))
	})
	log.Fatal(http.ListenAndServe(listenAddress, mux))
}

func metricsServer(registry prometheus.Gatherer, host string, port int) {
	// Address to listen on for web interface and telemetry
	listenAddress := net.JoinHostPort(host, strconv.Itoa(port))

	glog.Infof("Starting metrics server: %s", listenAddress)

	mux := http.NewServeMux()

	mux.Handle("/debug/pprof/", http.HandlerFunc(pprof.Index))
	mux.Handle("/debug/pprof/cmdline", http.HandlerFunc(pprof.Cmdline))
	mux.Handle("/debug/pprof/profile", http.HandlerFunc(pprof.Profile))
	mux.Handle("/debug/pprof/symbol", http.HandlerFunc(pprof.Symbol))
	mux.Handle("/debug/pprof/trace", http.HandlerFunc(pprof.Trace))

	// Add metricsPath
	mux.Handle(metricsPath, promhttp.HandlerFor(registry, promhttp.HandlerOpts{ErrorLog: promLogger{}}))
	// Add healthzPath
	mux.HandleFunc(healthzPath, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	})
	// Add index
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
             <head><title>OpenShift Metrics Server</title></head>
             <body>
             <h1>OpenShift Metrics</h1>
			 <ul>
             <li><a href='` + metricsPath + `'>metrics</a></li>
             <li><a href='` + healthzPath + `'>healthz</a></li>
			 </ul>
             </body>
             </html>`))
	})
	log.Fatal(http.ListenAndServe(listenAddress, mux))
}

// registerCollectors creates and starts informers and initializes and
// registers metrics for collection.
func registerCollectors(registry prometheus.Registerer, enabledCollectors options.CollectorSet,  opts *options.Options) {

	activeCollectors := []string{}
	for c := range enabledCollectors {
		f, ok := ocollectors.AvailableCollectors[c]
		if ok {
			f(registry, opts)
			activeCollectors = append(activeCollectors, c)
		}
	}

	glog.Infof("Active collectors: %s", strings.Join(activeCollectors, ","))
}