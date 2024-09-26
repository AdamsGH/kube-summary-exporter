package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	corev1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	stats "k8s.io/kubelet/pkg/apis/stats/v1alpha1"

	// Support auth providers in kubeconfig files
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

var metricsNamespace = "kube_summary"

type PerNodeResult struct {
	NodeName string
	Summary  *stats.Summary
}

// collectSummaryMetrics collects metrics from a /stats/summary response
func collectSummaryMetrics(results []PerNodeResult, registry *prometheus.Registry) {
	var (
		containerLogsInodesFree = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: metricsNamespace,
			Name:      "container_logs_inodes_free",
			Help:      "Number of available Inodes for logs",
		},
			[]string{
				"node",
				"pod",
				"namespace",
				"name",
			},
		)
		containerLogsInodes = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: metricsNamespace,
			Name:      "container_logs_inodes",
			Help:      "Number of Inodes for logs",
		},
			[]string{
				"node",
				"pod",
				"namespace",
				"name",
			},
		)
		containerLogsInodesUsed = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: metricsNamespace,
			Name:      "container_logs_inodes_used",
			Help:      "Number of used Inodes for logs",
		},
			[]string{
				"node",
				"pod",
				"namespace",
				"name",
			},
		)
		containerLogsAvailableBytes = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: metricsNamespace,
			Name:      "container_logs_available_bytes",
			Help:      "Number of bytes that aren't consumed by the container logs",
		},
			[]string{
				"node",
				"pod",
				"namespace",
				"name",
			},
		)
		containerLogsCapacityBytes = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: metricsNamespace,
			Name:      "container_logs_capacity_bytes",
			Help:      "Number of bytes that can be consumed by the container logs",
		},
			[]string{
				"node",
				"pod",
				"namespace",
				"name",
			},
		)
		containerLogsUsedBytes = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: metricsNamespace,
			Name:      "container_logs_used_bytes",
			Help:      "Number of bytes that are consumed by the container logs",
		},
			[]string{
				"node",
				"pod",
				"namespace",
				"name",
			},
		)
		containerRootFsInodesFree = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: metricsNamespace,
			Name:      "container_rootfs_inodes_free",
			Help:      "Number of available Inodes",
		},
			[]string{
				"node",
				"pod",
				"namespace",
				"name",
			},
		)
		containerRootFsInodes = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: metricsNamespace,
			Name:      "container_rootfs_inodes",
			Help:      "Number of Inodes",
		},
			[]string{
				"node",
				"pod",
				"namespace",
				"name",
			},
		)
		containerRootFsInodesUsed = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: metricsNamespace,
			Name:      "container_rootfs_inodes_used",
			Help:      "Number of used Inodes",
		},
			[]string{
				"node",
				"pod",
				"namespace",
				"name",
			},
		)
		containerRootFsAvailableBytes = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: metricsNamespace,
			Name:      "container_rootfs_available_bytes",
			Help:      "Number of bytes that aren't consumed by the container",
		},
			[]string{
				"node",
				"pod",
				"namespace",
				"name",
			},
		)
		containerRootFsCapacityBytes = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: metricsNamespace,
			Name:      "container_rootfs_capacity_bytes",
			Help:      "Number of bytes that can be consumed by the container",
		},
			[]string{
				"node",
				"pod",
				"namespace",
				"name",
			},
		)
		containerRootFsUsedBytes = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: metricsNamespace,
			Name:      "container_rootfs_used_bytes",
			Help:      "Number of bytes that are consumed by the container",
		},
			[]string{
				"node",
				"pod",
				"namespace",
				"name",
			},
		)
		podEphemeralStorageAvailableBytes = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: metricsNamespace,
			Name:      "pod_ephemeral_storage_available_bytes",
			Help:      "Number of bytes of Ephemeral storage that aren't consumed by the pod",
		},
			[]string{
				"node",
				"pod",
				"namespace",
			},
		)
		podEphemeralStorageCapacityBytes = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: metricsNamespace,
			Name:      "pod_ephemeral_storage_capacity_bytes",
			Help:      "Number of bytes of Ephemeral storage that can be consumed by the pod",
		},
			[]string{
				"node",
				"pod",
				"namespace",
			},
		)
		podEphemeralStorageUsedBytes = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: metricsNamespace,
			Name:      "pod_ephemeral_storage_used_bytes",
			Help:      "Number of bytes of Ephemeral storage that are consumed by the pod",
		},
			[]string{
				"node",
				"pod",
				"namespace",
			},
		)
		podEphemeralStorageInodesFree = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: metricsNamespace,
			Name:      "pod_ephemeral_storage_inodes_free",
			Help:      "Number of available Inodes for pod Ephemeral storage",
		},
			[]string{
				"node",
				"pod",
				"namespace",
			},
		)
		podEphemeralStorageInodes = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: metricsNamespace,
			Name:      "pod_ephemeral_storage_inodes",
			Help:      "Number of Inodes for pod Ephemeral storage",
		},
			[]string{
				"node",
				"pod",
				"namespace",
			},
		)
		podEphemeralStorageInodesUsed = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: metricsNamespace,
			Name:      "pod_ephemeral_storage_inodes_used",
			Help:      "Number of used Inodes for pod Ephemeral storage",
		},
			[]string{
				"node",
				"pod",
				"namespace",
			},
		)
		nodeRuntimeImageFSAvailableBytes = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: metricsNamespace,
			Name:      "node_runtime_imagefs_available_bytes",
			Help:      "Number of bytes of node Runtime ImageFS that aren't consumed",
		},
			[]string{
				"node",
			},
		)
		nodeRuntimeImageFSCapacityBytes = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: metricsNamespace,
			Name:      "node_runtime_imagefs_capacity_bytes",
			Help:      "Number of bytes of node Runtime ImageFS that can be consumed",
		},
			[]string{
				"node",
			},
		)
		nodeRuntimeImageFSUsedBytes = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: metricsNamespace,
			Name:      "node_runtime_imagefs_used_bytes",
			Help:      "Number of bytes of node Runtime ImageFS that are consumed",
		},
			[]string{
				"node",
			},
		)
		nodeRuntimeImageFSInodesFree = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: metricsNamespace,
			Name:      "node_runtime_imagefs_inodes_free",
			Help:      "Number of available Inodes for node Runtime ImageFS",
		},
			[]string{
				"node",
			},
		)
		nodeRuntimeImageFSInodes = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: metricsNamespace,
			Name:      "node_runtime_imagefs_inodes",
			Help:      "Number of Inodes for node Runtime ImageFS",
		},
			[]string{
				"node",
			},
		)
		nodeRuntimeImageFSInodesUsed = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: metricsNamespace,
			Name:      "node_runtime_imagefs_inodes_used",
			Help:      "Number of used Inodes for node Runtime ImageFS",
		},
			[]string{
				"node",
			},
		)
	)
	registry.MustRegister(
		containerLogsInodesFree,
		containerLogsInodes,
		containerLogsInodesUsed,
		containerLogsAvailableBytes,
		containerLogsCapacityBytes,
		containerLogsUsedBytes,
		containerRootFsInodesFree,
		containerRootFsInodes,
		containerRootFsInodesUsed,
		containerRootFsAvailableBytes,
		containerRootFsCapacityBytes,
		containerRootFsUsedBytes,
		podEphemeralStorageAvailableBytes,
		podEphemeralStorageCapacityBytes,
		podEphemeralStorageUsedBytes,
		podEphemeralStorageInodesFree,
		podEphemeralStorageInodes,
		podEphemeralStorageInodesUsed,
		nodeRuntimeImageFSAvailableBytes,
		nodeRuntimeImageFSCapacityBytes,
		nodeRuntimeImageFSUsedBytes,
		nodeRuntimeImageFSInodesFree,
		nodeRuntimeImageFSInodes,
		nodeRuntimeImageFSInodesUsed,
	)

	for _, entry := range results {
		nodeName := entry.NodeName
		summary := entry.Summary

		for _, pod := range summary.Pods {
			for _, container := range pod.Containers {
				if logs := container.Logs; logs != nil {
					if inodesFree := logs.InodesFree; inodesFree != nil {
						containerLogsInodesFree.WithLabelValues(nodeName, pod.PodRef.Name, pod.PodRef.Namespace, container.Name).Set(float64(*inodesFree))
					}
					if inodes := logs.Inodes; inodes != nil {
						containerLogsInodes.WithLabelValues(nodeName, pod.PodRef.Name, pod.PodRef.Namespace, container.Name).Set(float64(*inodes))
					}
					if inodesUsed := logs.InodesUsed; inodesUsed != nil {
						containerLogsInodesUsed.WithLabelValues(nodeName, pod.PodRef.Name, pod.PodRef.Namespace, container.Name).Set(float64(*inodesUsed))
					}
					if availableBytes := logs.AvailableBytes; availableBytes != nil {
						containerLogsAvailableBytes.WithLabelValues(nodeName, pod.PodRef.Name, pod.PodRef.Namespace, container.Name).Set(float64(*availableBytes))
					}
					if capacityBytes := logs.CapacityBytes; capacityBytes != nil {
						containerLogsCapacityBytes.WithLabelValues(nodeName, pod.PodRef.Name, pod.PodRef.Namespace, container.Name).Set(float64(*capacityBytes))
					}
					if usedBytes := logs.UsedBytes; usedBytes != nil {
						containerLogsUsedBytes.WithLabelValues(nodeName, pod.PodRef.Name, pod.PodRef.Namespace, container.Name).Set(float64(*usedBytes))
					}
				}
				if rootfs := container.Rootfs; rootfs != nil {
					if inodesFree := rootfs.InodesFree; inodesFree != nil {
						containerRootFsInodesFree.WithLabelValues(nodeName, pod.PodRef.Name, pod.PodRef.Namespace, container.Name).Set(float64(*inodesFree))
					}
					if inodes := rootfs.Inodes; inodes != nil {
						containerRootFsInodes.WithLabelValues(nodeName, pod.PodRef.Name, pod.PodRef.Namespace, container.Name).Set(float64(*inodes))
					}
					if inodesUsed := rootfs.InodesUsed; inodesUsed != nil {
						containerRootFsInodesUsed.WithLabelValues(nodeName, pod.PodRef.Name, pod.PodRef.Namespace, container.Name).Set(float64(*inodesUsed))
					}
					if availableBytes := rootfs.AvailableBytes; availableBytes != nil {
						containerRootFsAvailableBytes.WithLabelValues(nodeName, pod.PodRef.Name, pod.PodRef.Namespace, container.Name).Set(float64(*availableBytes))
					}
					if capacityBytes := rootfs.CapacityBytes; capacityBytes != nil {
						containerRootFsCapacityBytes.WithLabelValues(nodeName, pod.PodRef.Name, pod.PodRef.Namespace, container.Name).Set(float64(*capacityBytes))
					}
					if usedBytes := rootfs.UsedBytes; usedBytes != nil {
						containerRootFsUsedBytes.WithLabelValues(nodeName, pod.PodRef.Name, pod.PodRef.Namespace, container.Name).Set(float64(*usedBytes))
					}
				}
			}

			if ephemeralStorage := pod.EphemeralStorage; ephemeralStorage != nil {
				if ephemeralStorage.AvailableBytes != nil {
					podEphemeralStorageAvailableBytes.WithLabelValues(nodeName, pod.PodRef.Name, pod.PodRef.Namespace).Set(float64(*ephemeralStorage.AvailableBytes))
				}
				if ephemeralStorage.CapacityBytes != nil {
					podEphemeralStorageCapacityBytes.WithLabelValues(nodeName, pod.PodRef.Name, pod.PodRef.Namespace).Set(float64(*ephemeralStorage.CapacityBytes))
				}
				if ephemeralStorage.UsedBytes != nil {
					podEphemeralStorageUsedBytes.WithLabelValues(nodeName, pod.PodRef.Name, pod.PodRef.Namespace).Set(float64(*ephemeralStorage.UsedBytes))
				}
				if ephemeralStorage.InodesFree != nil {
					podEphemeralStorageInodesFree.WithLabelValues(nodeName, pod.PodRef.Name, pod.PodRef.Namespace).Set(float64(*ephemeralStorage.InodesFree))
				}
				if ephemeralStorage.Inodes != nil {
					podEphemeralStorageInodes.WithLabelValues(nodeName, pod.PodRef.Name, pod.PodRef.Namespace).Set(float64(*ephemeralStorage.Inodes))
				}
				if ephemeralStorage.InodesUsed != nil {
					podEphemeralStorageInodesUsed.WithLabelValues(nodeName, pod.PodRef.Name, pod.PodRef.Namespace).Set(float64(*ephemeralStorage.InodesUsed))
				}
			}
		}

		if runtime := summary.Node.Runtime; runtime != nil {
			if runtime.ImageFs.AvailableBytes != nil {
				nodeRuntimeImageFSAvailableBytes.WithLabelValues(nodeName).Set(float64(*runtime.ImageFs.AvailableBytes))
			}
			if runtime.ImageFs.CapacityBytes != nil {
				nodeRuntimeImageFSCapacityBytes.WithLabelValues(nodeName).Set(float64(*runtime.ImageFs.CapacityBytes))
			}
			if runtime.ImageFs.UsedBytes != nil {
				nodeRuntimeImageFSUsedBytes.WithLabelValues(nodeName).Set(float64(*runtime.ImageFs.UsedBytes))
			}
			if runtime.ImageFs.InodesFree != nil {
				nodeRuntimeImageFSInodesFree.WithLabelValues(nodeName).Set(float64(*runtime.ImageFs.InodesFree))
			}
			if runtime.ImageFs.Inodes != nil {
				nodeRuntimeImageFSInodes.WithLabelValues(nodeName).Set(float64(*runtime.ImageFs.Inodes))
			}
			if runtime.ImageFs.InodesUsed != nil {
				nodeRuntimeImageFSInodesUsed.WithLabelValues(nodeName).Set(float64(*runtime.ImageFs.InodesUsed))
			}
		}
	}
}

// handleMetricsCollection is a generic handler for collecting metrics
func handleMetricsCollection(w http.ResponseWriter, r *http.Request, kubeClient *kubernetes.Clientset, nodeSelector func(context.Context, *kubernetes.Clientset) ([]PerNodeResult, error)) {
	ctx, cancel := getTimeoutContext(r)
	defer cancel()

	results, err := nodeSelector(ctx, kubeClient)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error collecting node stats: %v", err), http.StatusInternalServerError)
		return
	}

	registry := prometheus.NewRegistry()
	collectSummaryMetrics(results, registry)
	h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	h.ServeHTTP(w, r)
}

// allNodesSelector selects all nodes in the cluster
func allNodesSelector(ctx context.Context, kubeClient *kubernetes.Clientset) ([]PerNodeResult, error) {
	nodes, err := kubeClient.CoreV1().Nodes().List(ctx, meta_v1.ListOptions{}) // Использование meta_v1.ListOptions
	if err != nil {
		return nil, fmt.Errorf("error enumerating nodes: %v", err)
	}

	return collectNodeStats(ctx, kubeClient, nodes.Items)
}

// singleNodeSelector selects a single node by name
func singleNodeSelector(nodeName string) func(context.Context, *kubernetes.Clientset) ([]PerNodeResult, error) {
	return func(ctx context.Context, kubeClient *kubernetes.Clientset) ([]PerNodeResult, error) {
		node, err := kubeClient.CoreV1().Nodes().Get(ctx, nodeName, meta_v1.GetOptions{}) // Использование meta_v1.GetOptions
		if err != nil {
			return nil, fmt.Errorf("error getting node %s: %v", nodeName, err)
		}

		return collectNodeStats(ctx, kubeClient, []corev1.Node{*node}) // Использование corev1.Node
	}
}

// collectNodeStats collects stats for the given nodes
func collectNodeStats(ctx context.Context, kubeClient *kubernetes.Clientset, nodes []corev1.Node) ([]PerNodeResult, error) {
	var results []PerNodeResult

	for _, node := range nodes {
		summary, err := getNodeSummary(ctx, kubeClient, node.Name)
		if err != nil {
			return nil, err
		}

		results = append(results, PerNodeResult{
			NodeName: node.Name,
			Summary:  summary,
		})
	}

	return results, nil
}

// getNodeSummary retrieves the summary for a single node
func getNodeSummary(ctx context.Context, kubeClient *kubernetes.Clientset, nodeName string) (*stats.Summary, error) {
	req := kubeClient.CoreV1().RESTClient().Get().Resource("nodes").Name(nodeName).SubResource("proxy").Suffix("stats/summary")
	resp, err := req.DoRaw(ctx)
	if err != nil {
		return nil, fmt.Errorf("error querying /stats/summary for %s: %v", nodeName, err)
	}

	summary := &stats.Summary{}
	if err := json.Unmarshal(resp, summary); err != nil {
		return nil, fmt.Errorf("error unmarshaling /stats/summary response for %s: %v", nodeName, err)
	}

	return summary, nil
}

// getTimeoutContext returns a context with timeout based on the X-Prometheus-Scrape-Timeout-Seconds header
func getTimeoutContext(r *http.Request) (context.Context, context.CancelFunc) {
	if v := r.Header.Get("X-Prometheus-Scrape-Timeout-Seconds"); v != "" {
		timeoutSeconds, err := strconv.ParseFloat(v, 64)
		if err == nil {
			return context.WithTimeout(r.Context(), time.Duration(timeoutSeconds*float64(time.Second)))
		}
	}
	return context.WithCancel(r.Context())
}

// newKubeClient returns a Kubernetes client (clientset) from the supplied
// kubeconfig path, the KUBECONFIG environment variable, the default config file
// location ($HOME/.kube/config) or from the in-cluster service account environment.
func newKubeClient(path string) (*kubernetes.Clientset, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if path != "" {
		loadingRules.ExplicitPath = path
	}

	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules,
		&clientcmd.ConfigOverrides{},
	)

	config, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, err
	}

	return kubernetes.NewForConfig(config)
}

var (
	flagListenAddress  = flag.String("listen-address", ":9779", "Listen address")
	flagKubeConfigPath = flag.String("kubeconfig", "", "Path of a kubeconfig file, if not provided the app will try $KUBECONFIG, $HOME/.kube/config or in cluster config")
)

func main() {
	flag.Parse()

	kubeClient, err := newKubeClient(*flagKubeConfigPath)
	if err != nil {
		fmt.Printf("[Error] Cannot create kube client: %v", err)
		os.Exit(1)
	}

	r := mux.NewRouter()
	r.HandleFunc("/nodes", func(w http.ResponseWriter, r *http.Request) {
		handleMetricsCollection(w, r, kubeClient, allNodesSelector)
	})
	r.HandleFunc("/node/{node}", func(w http.ResponseWriter, r *http.Request) {
		nodeName := mux.Vars(r)["node"]
		handleMetricsCollection(w, r, kubeClient, singleNodeSelector(nodeName))
	})
	r.Handle("/metrics", promhttp.Handler())
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html>
    <head><title>Kube Summary Exporter</title></head>
    <body>
        <h1>Kube Summary Exporter</h1>
        <p><a href="/nodes">Retrieve metrics for all nodes</a></p>
        <p><a href="/node/example-node">Retrieve metrics for 'example-node'</a></p>
        <p><a href="/metrics">Metrics</a></p>
    </body>
</html>`))
	})

	fmt.Printf("Listening on %s\n", *flagListenAddress)
	fmt.Printf("error: %v\n", http.ListenAndServe(*flagListenAddress, r))
}
