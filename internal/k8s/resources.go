package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"
)

// ResourcePort represents a port exposed by a k8s resource
type ResourcePort struct {
	Name          string
	ContainerPort int
	Port          int // for services: the service port
}

// FetchServices returns service names in the given namespace.
func FetchServices(kubeconfigPath, kubeContext, namespace string) ([]string, error) {
	if DemoMode {
		if svcs, ok := demoServices[namespace]; ok {
			return svcs, nil
		}
		return []string{}, nil
	}

	return fetchResourceNames(kubeconfigPath, kubeContext, namespace, "services")
}

// FetchPods returns pod names in the given namespace.
func FetchPods(kubeconfigPath, kubeContext, namespace string) ([]string, error) {
	if DemoMode {
		if pods, ok := demoPods[namespace]; ok {
			return pods, nil
		}
		return []string{}, nil
	}

	return fetchResourceNames(kubeconfigPath, kubeContext, namespace, "pods")
}

// FetchDeployments returns deployment names in the given namespace.
func FetchDeployments(kubeconfigPath, kubeContext, namespace string) ([]string, error) {
	if DemoMode {
		if deps, ok := demoDeployments[namespace]; ok {
			return deps, nil
		}
		return []string{}, nil
	}

	return fetchResourceNames(kubeconfigPath, kubeContext, namespace, "deployments")
}

func fetchResourceNames(kubeconfigPath, kubeContext, namespace, resource string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	args := buildKubectlArgs(kubeconfigPath, kubeContext)
	args = append(args, "get", resource, "-n", namespace,
		"-o", "jsonpath={.items[*].metadata.name}")

	out, err := exec.CommandContext(ctx, "kubectl", args...).Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("kubectl timed out after 10s")
		}
		if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) > 0 {
			return nil, fmt.Errorf("kubectl: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, err
	}

	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return nil, nil
	}

	names := strings.Fields(raw)
	sort.Strings(names)
	return names, nil
}

// FetchServicePorts returns ports exposed by a service.
func FetchServicePorts(kubeconfigPath, kubeContext, namespace, name string) ([]ResourcePort, error) {
	if DemoMode {
		return getDemoPorts(name), nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	args := buildKubectlArgs(kubeconfigPath, kubeContext)
	args = append(args, "get", "svc", name, "-n", namespace, "-o", "json")

	out, err := exec.CommandContext(ctx, "kubectl", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("fetching service ports: %w", err)
	}

	var svc struct {
		Spec struct {
			Ports []struct {
				Name       string `json:"name"`
				Port       int    `json:"port"`
				TargetPort json.RawMessage `json:"targetPort"`
			} `json:"ports"`
		} `json:"spec"`
	}
	if err := json.Unmarshal(out, &svc); err != nil {
		return nil, fmt.Errorf("parsing service JSON: %w", err)
	}

	var ports []ResourcePort
	for _, p := range svc.Spec.Ports {
		ports = append(ports, ResourcePort{
			Name: p.Name,
			Port: p.Port,
		})
	}
	return ports, nil
}

// FetchPodPorts returns container ports exposed by a pod.
func FetchPodPorts(kubeconfigPath, kubeContext, namespace, name string) ([]ResourcePort, error) {
	if DemoMode {
		return getDemoPorts(name), nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	args := buildKubectlArgs(kubeconfigPath, kubeContext)
	args = append(args, "get", "pod", name, "-n", namespace, "-o", "json")

	out, err := exec.CommandContext(ctx, "kubectl", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("fetching pod ports: %w", err)
	}

	var pod struct {
		Spec struct {
			Containers []struct {
				Ports []struct {
					Name          string `json:"name"`
					ContainerPort int    `json:"containerPort"`
				} `json:"ports"`
			} `json:"containers"`
		} `json:"spec"`
	}
	if err := json.Unmarshal(out, &pod); err != nil {
		return nil, fmt.Errorf("parsing pod JSON: %w", err)
	}

	var ports []ResourcePort
	for _, c := range pod.Spec.Containers {
		for _, p := range c.Ports {
			ports = append(ports, ResourcePort{
				Name:          p.Name,
				ContainerPort: p.ContainerPort,
				Port:          p.ContainerPort,
			})
		}
	}
	return ports, nil
}

// FetchDeploymentPorts returns container ports from a deployment's pod template.
func FetchDeploymentPorts(kubeconfigPath, kubeContext, namespace, name string) ([]ResourcePort, error) {
	if DemoMode {
		return getDemoPorts(name), nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	args := buildKubectlArgs(kubeconfigPath, kubeContext)
	args = append(args, "get", "deployment", name, "-n", namespace, "-o", "json")

	out, err := exec.CommandContext(ctx, "kubectl", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("fetching deployment ports: %w", err)
	}

	var deploy struct {
		Spec struct {
			Template struct {
				Spec struct {
					Containers []struct {
						Ports []struct {
							Name          string `json:"name"`
							ContainerPort int    `json:"containerPort"`
						} `json:"ports"`
					} `json:"containers"`
				} `json:"spec"`
			} `json:"template"`
		} `json:"spec"`
	}
	if err := json.Unmarshal(out, &deploy); err != nil {
		return nil, fmt.Errorf("parsing deployment JSON: %w", err)
	}

	var ports []ResourcePort
	for _, c := range deploy.Spec.Template.Spec.Containers {
		for _, p := range c.Ports {
			ports = append(ports, ResourcePort{
				Name:          p.Name,
				ContainerPort: p.ContainerPort,
				Port:          p.ContainerPort,
			})
		}
	}
	return ports, nil
}

func buildKubectlArgs(kubeconfigPath, kubeContext string) []string {
	args := []string{"--kubeconfig", kubeconfigPath}
	if kubeContext != "" {
		args = append(args, "--context", kubeContext)
	}
	return args
}
