package k8s

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/streed/smol-cluster/gateway/internal/config"
	"github.com/streed/smol-cluster/gateway/internal/models"
)

func BuildPodSpec(podName string, ws models.Workstream, repo models.Repository, cfg *config.Config) *corev1.Pod {
	labels := map[string]string{
		"app":                    "smol-agent",
		"smol-cluster/workstream": ws.ID.String(),
		"smol-cluster/repo":      repo.ID.String(),
	}

	// Default resource limits
	cpuRequest := "500m"
	cpuLimit := "2000m"
	memRequest := "1Gi"
	memLimit := "4Gi"
	diskSize := "10Gi"

	if repo.Config != nil && repo.Config.ResourceLimits != nil {
		rl := repo.Config.ResourceLimits
		if rl.CPURequest != "" {
			cpuRequest = rl.CPURequest
		}
		if rl.CPULimit != "" {
			cpuLimit = rl.CPULimit
		}
		if rl.MemoryRequest != "" {
			memRequest = rl.MemoryRequest
		}
		if rl.MemoryLimit != "" {
			memLimit = rl.MemoryLimit
		}
		if rl.DiskSize != "" {
			diskSize = rl.DiskSize
		}
	}

	// LLM config
	llmURL := cfg.LLMApiURL
	llmKey := cfg.LLMApiKey
	llmModel := cfg.LLMModel
	if ws.LLMConfig != nil {
		if ws.LLMConfig.APIURL != "" {
			llmURL = ws.LLMConfig.APIURL
		}
		if ws.LLMConfig.APIKey != "" {
			llmKey = ws.LLMConfig.APIKey
		}
		if ws.LLMConfig.Model != "" {
			llmModel = ws.LLMConfig.Model
		}
	}

	// Build env vars
	envVars := []corev1.EnvVar{
		{Name: "REPO_URL", Value: repo.GitURL},
		{Name: "BRANCH_NAME", Value: ws.BranchName},
		{Name: "LLM_API_URL", Value: llmURL},
		{Name: "LLM_API_KEY", Value: llmKey},
		{Name: "LLM_MODEL", Value: llmModel},
		{Name: "WORKSTREAM_ID", Value: ws.ID.String()},
		{Name: "GATEWAY_URL", Value: fmt.Sprintf("http://smol-cluster-gateway.%s.svc.cluster.local:8080", cfg.K8sNamespace)},
		{Name: "ACP_PORT", Value: "8021"},
		{Name: "BRIDGE_PORT", Value: "8022"},
	}

	// Add repo-level env vars
	if repo.Config != nil {
		for k, v := range repo.Config.EnvVars {
			envVars = append(envVars, corev1.EnvVar{Name: k, Value: v})
		}
	}

	// Agent prompt
	agentPrompt := ws.Description
	if repo.Config != nil && repo.Config.AgentPrompt != "" && agentPrompt == "" {
		agentPrompt = repo.Config.AgentPrompt
	}
	envVars = append(envVars, corev1.EnvVar{Name: "AGENT_PROMPT", Value: agentPrompt})

	// Setup commands as JSON
	if repo.Config != nil && len(repo.Config.SetupCommands) > 0 {
		// Pass as comma-separated for shell parsing
		cmds := ""
		for i, c := range repo.Config.SetupCommands {
			if i > 0 {
				cmds += ";"
			}
			cmds += c
		}
		envVars = append(envVars, corev1.EnvVar{Name: "SETUP_COMMANDS", Value: cmds})
	}

	// Build container ports
	containerPorts := []corev1.ContainerPort{
		{Name: "acp", ContainerPort: 8021, Protocol: corev1.ProtocolTCP},
		{Name: "bridge", ContainerPort: 8022, Protocol: corev1.ProtocolTCP},
	}
	for _, pm := range ws.PortMappings {
		containerPorts = append(containerPorts, corev1.ContainerPort{
			Name:          pm.Name,
			ContainerPort: int32(pm.ContainerPort),
			Protocol:      corev1.ProtocolTCP,
		})
	}

	privileged := true

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: cfg.K8sNamespace,
			Labels:    labels,
			Annotations: map[string]string{
				"smol-cluster/workstream-name": ws.Name,
				"smol-cluster/repo-name":       repo.Name,
			},
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Volumes: []corev1.Volume{
				{
					Name: "workspace",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{
							SizeLimit: resourcePtr(resource.MustParse(diskSize)),
						},
					},
				},
				{
					Name: "docker-socket",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			},
			// Docker-in-Docker sidecar for repos using docker-compose
			InitContainers: []corev1.Container{},
			Containers: []corev1.Container{
				{
					Name:  "agent",
					Image: cfg.AgentImage,
					Env:   envVars,
					Ports: containerPorts,
					VolumeMounts: []corev1.VolumeMount{
						{Name: "workspace", MountPath: "/workspace"},
						{Name: "docker-socket", MountPath: "/var/run"},
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse(cpuRequest),
							corev1.ResourceMemory: resource.MustParse(memRequest),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse(cpuLimit),
							corev1.ResourceMemory: resource.MustParse(memLimit),
						},
					},
					ReadinessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							HTTPGet: &corev1.HTTPGetAction{
								Path: "/health",
								Port: intstr.FromInt(8022),
							},
						},
						InitialDelaySeconds: 10,
						PeriodSeconds:       5,
					},
				},
				// Docker-in-Docker sidecar for repos that run docker-compose
				{
					Name:  "dind",
					Image: "docker:27-dind",
					SecurityContext: &corev1.SecurityContext{
						Privileged: &privileged,
					},
					Env: []corev1.EnvVar{
						{Name: "DOCKER_TLS_CERTDIR", Value: ""},
					},
					VolumeMounts: []corev1.VolumeMount{
						{Name: "workspace", MountPath: "/workspace"},
						{Name: "docker-socket", MountPath: "/var/run"},
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("250m"),
							corev1.ResourceMemory: resource.MustParse("512Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1000m"),
							corev1.ResourceMemory: resource.MustParse("2Gi"),
						},
					},
				},
			},
		},
	}

	return pod
}

func BuildServiceSpec(serviceName, podName string, ws models.Workstream) *corev1.Service {
	ports := []corev1.ServicePort{
		{Name: "acp", Port: 8021, TargetPort: intstr.FromInt(8021), Protocol: corev1.ProtocolTCP},
		{Name: "bridge", Port: 8022, TargetPort: intstr.FromInt(8022), Protocol: corev1.ProtocolTCP},
	}
	for _, pm := range ws.PortMappings {
		ports = append(ports, corev1.ServicePort{
			Name:       pm.Name,
			Port:       int32(pm.ContainerPort),
			TargetPort: intstr.FromInt(pm.ContainerPort),
			Protocol:   corev1.ProtocolTCP,
		})
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: serviceName,
			Labels: map[string]string{
				"app":                    "smol-agent",
				"smol-cluster/workstream": ws.ID.String(),
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"smol-cluster/workstream": ws.ID.String(),
			},
			Ports: ports,
			Type:  corev1.ServiceTypeClusterIP,
		},
	}
}

func resourcePtr(r resource.Quantity) *resource.Quantity {
	return &r
}
