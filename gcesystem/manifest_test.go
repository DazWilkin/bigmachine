package gcesystem

import (
	"testing"
)

// TODO(dazwilkin) Enrich Instance test w/ Volume[Mount]s
func TestBigMachineManifest(t *testing.T) {
	m := &Manifest{
		Spec: Spec{
			Containers: []Container{
				Container{
					Name:          "test",
					Image:         "test",
					Stdin:         false,
					TTY:           false,
					RestartPolicy: "Always",
					Args: []string{
						"-log=debug",
					},
					Env: []Env{
						Env{
							Name:  "BIGMACHINE_MODE",
							Value: "machine",
						},
						Env{
							Name:  "BIGMACHINE_SYSTEM",
							Value: "gce",
						},
						Env{
							Name:  "BIGMACHINE_ADDR",
							Value: ":8443",
						},
					},
				},
			},
		},
	}
	got, _ := m.String()

	// Must use spaces not tabs: pay care the editor replaces spaces with tabs
	// Must end with an newline
	want := `spec:
  containers:
  - name: test
    image: test
    restartPolicy: Always
    args:
    - -log=debug
    env:
    - name: BIGMACHINE_MODE
      value: machine
    - name: BIGMACHINE_SYSTEM
      value: gce
    - name: BIGMACHINE_ADDR
      value: :8443
`
	if got != want {
		t.Errorf("got:\n%swant:\n%s", got, want)
	}
}
func TestBigMachineManifestWithVolume(t *testing.T) {
	m := &Manifest{
		Spec: Spec{
			Containers: []Container{
				Container{
					Name:          "test",
					Image:         "test",
					Stdin:         false,
					TTY:           false,
					RestartPolicy: "Always",
					Args: []string{
						"-log=debug",
					},
					Env: []Env{
						Env{
							Name:  "BIGMACHINE_MODE",
							Value: "machine",
						},
						Env{
							Name:  "BIGMACHINE_SYSTEM",
							Value: "gce",
						},
						Env{
							Name:  "BIGMACHINE_ADDR",
							Value: ":8443",
						},
					},
					VolumeMounts: []VolumeMount{
						VolumeMount{
							Name:      "tmpfs",
							MountPath: "/tmp",
						},
					},
				},
			},
			Volumes: []Volume{
				Volume{
					Name: "tmpfs",
					EmptyDir: EmptyDir{
						Medium: "Memory",
					},
				},
			},
		},
	}
	got, _ := m.String()

	// Must use spaces not tabs: pay care the editor replaces spaces with tabs
	// Must end with an newline
	want := `spec:
  containers:
  - name: test
    image: test
    restartPolicy: Always
    args:
    - -log=debug
    env:
    - name: BIGMACHINE_MODE
      value: machine
    - name: BIGMACHINE_SYSTEM
      value: gce
    - name: BIGMACHINE_ADDR
      value: :8443
    volumeMounts:
    - name: tmpfs
      mountPath: /tmp
  volumes:
  - name: tmpfs
    emptyDir:
      medium: Memory
`
	if got != want {
		t.Errorf("got:\n%swant:\n%s", got, want)
	}
}
func TestNotOneContainer(t *testing.T) {
	t.Run("zero", func(t *testing.T) {
		m := &Manifest{
			Spec: Spec{
				Containers: []Container{},
			},
		}
		_, err := m.String()
		if err == nil {
			t.Errorf("Expected error: zero containers is not permitted")
		}
	})
	t.Run("multiple", func(t *testing.T) {
		m := &Manifest{
			Spec: Spec{
				Containers: []Container{
					Container{
						Name:  "name",
						Image: "image",
					},
					Container{
						Name:  "name",
						Image: "image",
					},
				},
			},
		}
		_, err := m.String()
		if err == nil {
			t.Errorf("Expected error: multiple containers is not permitted")
		}

	})
}
