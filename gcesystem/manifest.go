package gcesystem

import (
	"fmt"

	// TODO(dazwilkin) Probable that this would work using JSON too but YAML is used by GCE by default
	"gopkg.in/yaml.v2"
)

type Manifest struct {
	Spec Spec
}

// Runtime restriction that only one container is permitted
type Spec struct {
	Containers []Container
	Volumes    []Volume
}
type Container struct {
	Name          string        `yaml:"name"`
	Image         string        `yaml:"image"`
	Stdin         bool          `yaml:"stdin,omitempty"`
	TTY           bool          `yaml:"tty,omitempty"`
	RestartPolicy string        `yaml:"restartPolicy,omitempty"`
	Args          []string      `yaml:"args,omitempty"`
	Env           []Env         `yaml:"env,omitempty"`
	VolumeMounts  []VolumeMount `yaml:"volumeMounts,omitempty"`
}
type Env struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}
type VolumeMount struct {
	Name      string `yaml:"name"`
	MountPath string `yaml:"mountPath"`
}
type Volume struct {
	Name     string   `yaml:"name"`
	EmptyDir EmptyDir `yaml:"emptyDir"`
}
type EmptyDir struct {
	Medium string `yaml:"medium"`
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
