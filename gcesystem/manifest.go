package gcesystem

import (
	"fmt"

	// TODO(dazwilkin) Probable that this would work using JSON too but YAML is used by GCE by default
	"gopkg.in/yaml.v2"
)

type Manifest struct {
	Spec Spec `yaml:"spec"`
}

// Runtime restriction that only one container is permitted
type Spec struct {
	Containers []Container `yaml:"containers"`
	Volumes    []Volume    `yaml:"volumes,omitempty"`
}
type Container struct {
	Name            string          `yaml:"name"`
	Image           string          `yaml:"image"`
	SecurityContext SecurityContext `yaml:"securityContext,omitempty"`
	Stdin           bool            `yaml:"stdin,omitempty"`
	TTY             bool            `yaml:"tty,omitempty"`
	RestartPolicy   string          `yaml:"restartPolicy,omitempty"`
	Args            []string        `yaml:"args,omitempty"`
	Env             []Env           `yaml:"env,omitempty"`
	VolumeMounts    []VolumeMount   `yaml:"volumeMounts,omitempty"`
}
type SecurityContext struct {
	Privileged bool `yaml:"privileged"`
}
type Env struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}
type VolumeMount struct {
	Name      string `yaml:"name"`
	MountPath string `yaml:"mountPath"`
	ReadOnly  bool   `yaml:"readOnly"`
}
type Volume struct {
	Name string `yaml:"name"`
	// TODO(dazwilkin) This is not ideal Volume contains [EmtpyDir|HostPath]; to make life easier, assuming only one will be provided
	EmptyDir EmptyDir `yaml:"emptyDir,omitempty"`
	HostPath HostPath `yaml:"hostPath,omitempty"`
}
type EmptyDir struct {
	Medium string `yaml:"medium"`
}
type HostPath struct {
	Path string `yaml:"path"`
}

func (m *Manifest) String() (string, error) {
	l := len(m.Spec.Containers)
	if l == 0 || l > 1 {
		return "", fmt.Errorf("[gce] Spec must contain exactly one container")
	}
	s, err := yaml.Marshal(&m)
	if err != nil {
		return "", err
	}
	return string(s), err
}
