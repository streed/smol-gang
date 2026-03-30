package k8s

import (
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/streed/smol-gang/gateway/internal/config"
	"github.com/streed/smol-gang/gateway/internal/models"
)

func BuildPodSpec(podName string, ws models.Workstream, repo models.Repository, cfg *config.Config, extraEnv map[string]string) *corev1.Pod {
	labels := map[string]string{
		"app":                    "smol-agent",
		"smol-gang/workstream": ws.ID.String(),
		"smol-gang/repo":      repo.ID.String(),
	}

	// Default resource limits (kept low to allow multiple concurrent agents)
	cpuRequest := "250m"
	memRequest := "512Mi"
	memLimit := "2Gi"
	diskSize := "10Gi"

	if repo.Config != nil && repo.Config.ResourceLimits != nil {
		rl := repo.Config.ResourceLimits
		if rl.CPURequest != "" {
			cpuRequest = rl.CPURequest
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

	// LLM config — use the gateway's configured URL (e.g. Ollama cloud)
	llmURL := cfg.LLMApiURL
	llmKey := cfg.LLMApiKey
	llmModel := cfg.LLMModel
	llmProvider := cfg.LLMProvider
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
		if ws.LLMConfig.Provider != "" {
			llmProvider = ws.LLMConfig.Provider
		}
	}

	envVars := []corev1.EnvVar{
		{Name: "REPO_URL", Value: repo.GitURL},
		{Name: "BRANCH_NAME", Value: ws.BranchName},
		{Name: "LLM_API_URL", Value: llmURL},
		{Name: "LLM_API_KEY", Value: llmKey},
		{Name: "OLLAMA_API_KEY", Value: llmKey},
		{Name: "LLM_MODEL", Value: llmModel},
		{Name: "LLM_PROVIDER", Value: llmProvider},
		{Name: "WORKSTREAM_ID", Value: ws.ID.String()},
		{Name: "GATEWAY_URL", Value: fmt.Sprintf("http://10.0.2.2:%s", cfg.Port)},
		{Name: "BRIDGE_PORT", Value: "8022"},
		{Name: "GITHUB_OWNER", Value: repo.GitHubOwner},
		{Name: "GITHUB_REPO", Value: repo.GitHubRepo},
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

	// Extra env vars (GIT_TOKEN, GATEWAY_TOKEN, etc.)
	for k, v := range extraEnv {
		envVars = append(envVars, corev1.EnvVar{Name: k, Value: v})
	}

	// Setup commands as JSON
	if repo.Config != nil && len(repo.Config.SetupCommands) > 0 {
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
	agentPorts := []corev1.ContainerPort{
		{Name: "bridge", ContainerPort: 8022, Protocol: corev1.ProtocolTCP},
	}

	appPorts := []corev1.ContainerPort{}
	for _, pm := range ws.PortMappings {
		appPorts = append(appPorts, corev1.ContainerPort{
			Name:          pm.Name,
			ContainerPort: int32(pm.ContainerPort),
			Protocol:      corev1.ProtocolTCP,
		})
	}

	appEnvVars := []corev1.EnvVar{
		{Name: "WORKSPACE", Value: "/workspace/repo"},
	}
	if repo.Config != nil {
		for k, v := range repo.Config.EnvVars {
			appEnvVars = append(appEnvVars, corev1.EnvVar{Name: k, Value: v})
		}
		if len(repo.Config.SetupCommands) > 0 {
			cmds := ""
			for i, c := range repo.Config.SetupCommands {
				if i > 0 {
					cmds += ";"
				}
				cmds += c
			}
			appEnvVars = append(appEnvVars, corev1.EnvVar{Name: "APP_COMMANDS", Value: cmds})
		}
	}

	// Service/compose mode for app container
	if repo.Config != nil {
		if repo.Config.Compose != nil && repo.Config.Compose.Enabled {
			appEnvVars = append(appEnvVars, corev1.EnvVar{Name: "APP_MODE", Value: "compose"})
			composeFile := repo.Config.Compose.File
			if composeFile == "" {
				composeFile = "docker-compose.yml"
			}
			appEnvVars = append(appEnvVars, corev1.EnvVar{Name: "APP_COMPOSE_FILE", Value: composeFile})
		} else if len(repo.Config.Services) > 0 {
			appEnvVars = append(appEnvVars, corev1.EnvVar{Name: "APP_MODE", Value: "services"})
			servicesJSON, _ := json.Marshal(repo.Config.Services)
			appEnvVars = append(appEnvVars, corev1.EnvVar{Name: "APP_SERVICES", Value: string(servicesJSON)})
		}
	}

	privileged := true
	appRunnerImage := cfg.AppRunnerImage
	if repo.Config != nil && repo.Config.Environment != "" {
		if img, ok := models.EnvironmentImages[repo.Config.Environment]; ok && img != "" {
			appRunnerImage = img
		}
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: cfg.K8sNamespace,
			Labels:    labels,
			Annotations: map[string]string{
				"smol-gang/workstream-name": ws.Name,
				"smol-gang/repo-name":       repo.Name,
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
			InitContainers: []corev1.Container{},
			Containers: []corev1.Container{
				{
					Name:            "agent",
					Image:           cfg.AgentImage,
					ImagePullPolicy: corev1.PullIfNotPresent,
					Env:             envVars,
					Ports:           agentPorts,
					VolumeMounts: []corev1.VolumeMount{
						{Name: "workspace", MountPath: "/workspace"},
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("256Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("1Gi"),
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
				{
					Name:            "app",
					Image:           appRunnerImage,
					ImagePullPolicy: corev1.PullIfNotPresent,
					Env:             appEnvVars,
					Ports: appPorts,
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
							corev1.ResourceMemory: resource.MustParse(memLimit),
						},
					},
				},
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
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("128Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("512Mi"),
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
				"smol-gang/workstream": ws.ID.String(),
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"smol-gang/workstream": ws.ID.String(),
			},
			Ports: ports,
			Type:  corev1.ServiceTypeClusterIP,
		},
	}
}

func resourcePtr(r resource.Quantity) *resource.Quantity {
	return &r
}
