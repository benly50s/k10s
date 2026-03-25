package k8s

import (
	"strings"
)

// DemoMode is set to true when the --demo flag is passed.
// It bypasses all actual kubectl calls and returns fake data for UI testing.
var DemoMode bool

// Fake data for demo mode
var demoNamespaces = []string{"default", "backend", "frontend", "database", "monitoring"}

var demoServices = map[string][]string{
	"backend":    {"api-server", "auth-service", "worker-queue"},
	"frontend":   {"web-app", "admin-panel"},
	"database":   {"postgresql-master", "postgresql-replica", "redis-cache"},
	"monitoring": {"prometheus", "grafana", "alertmanager"},
	"default":    {"kubernetes"},
}

var demoDeployments = map[string][]string{
	"backend":    {"api-server-deploy", "auth-service-deploy"},
	"frontend":   {"web-app-deploy"},
	"monitoring": {"prometheus-server", "grafana-ui"},
}

var demoPods = map[string][]string{
	"backend":    {"api-server-7bc4b8-xk2m", "api-server-7bc4b8-zp9t", "auth-service-pod-1"},
	"frontend":   {"web-app-589fc-11ab", "web-app-589fc-22cd"},
	"database":   {"postgresql-master-0", "postgresql-replica-0", "redis-cache-0"},
	"monitoring": {"prometheus-pod-x", "grafana-pod-y"},
	"default":    {"utils-pod"},
}

func getDemoPorts(name string) []ResourcePort {
	if strings.Contains(name, "api") || strings.Contains(name, "web") || strings.Contains(name, "grafana") {
		return []ResourcePort{{Name: "http", Port: 8080}, {Name: "metrics", Port: 9090}}
	}
	if strings.Contains(name, "redis") {
		return []ResourcePort{{Name: "tcp", Port: 6379}}
	}
	if strings.Contains(name, "postgres") {
		return []ResourcePort{{Name: "tcp", Port: 5432}}
	}
	return []ResourcePort{{Name: "default", Port: 80}}
}
