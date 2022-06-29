/*
Copyright 2020 The Knative Authors

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

package v1beta1

import (
	"context"
	"path"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/apis/duck"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/tracker"
)

var kfbCondSet = apis.NewLivingConditionSet()

// GetGroupVersionKind returns the GroupVersionKind.
func (*KafkaBinding) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("KafkaBinding")
}

// GetConditionSet retrieves the condition set for this resource. Implements the KRShaped interface.
func (*KafkaBinding) GetConditionSet() apis.ConditionSet {
	return kfbCondSet
}

// GetUntypedSpec implements apis.HasSpec
func (s *KafkaBinding) GetUntypedSpec() interface{} {
	return s.Spec
}

// GetSubject implements psbinding.Bindable
func (sb *KafkaBinding) GetSubject() tracker.Reference {
	return sb.Spec.Subject
}

// GetBindingStatus implements psbinding.Bindable
func (sb *KafkaBinding) GetBindingStatus() duck.BindableStatus {
	return &sb.Status
}

// SetObservedGeneration implements psbinding.BindableStatus
func (sbs *KafkaBindingStatus) SetObservedGeneration(gen int64) {
	sbs.ObservedGeneration = gen
}

// InitializeConditions populates the KafkaBindingStatus's conditions field
// with all of its conditions configured to Unknown.
func (sbs *KafkaBindingStatus) InitializeConditions() {
	kfbCondSet.Manage(sbs).InitializeConditions()
}

// MarkBindingUnavailable marks the KafkaBinding's Ready condition to False with
// the provided reason and message.
func (sbs *KafkaBindingStatus) MarkBindingUnavailable(reason, message string) {
	kfbCondSet.Manage(sbs).MarkFalse(KafkaBindingConditionReady, reason, message)
}

// MarkBindingAvailable marks the KafkaBinding's Ready condition to True.
func (sbs *KafkaBindingStatus) MarkBindingAvailable() {
	kfbCondSet.Manage(sbs).MarkTrue(KafkaBindingConditionReady)
}

// Do implements psbinding.Bindable
func (kfb *KafkaBinding) Do(ctx context.Context, ps *duckv1.WithPod) {
	// First undo so that we can just unconditionally append below.
	kfb.Undo(ctx, ps)

	spec := ps.Spec.Template.Spec

	if kfb.Spec.Net.GSSAPI.Enable {

		if kfb.Spec.Net.GSSAPI.KeyTab.SecretKeyRef != nil {
			keytab := corev1.Volume{
				Name: "krb5.keytab",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: kfb.Spec.Net.GSSAPI.KeyTab.SecretKeyRef.Name,
						Items: []corev1.KeyToPath{
							{
								Key:  kfb.Spec.Net.GSSAPI.KeyTab.SecretKeyRef.Key,
								Path: "krb5.keytab",
							},
						},
					},
				},
			}
			spec.Volumes = append(spec.Volumes, keytab)
		}

		if kfb.Spec.Net.GSSAPI.Config.SecretKeyRef != nil {
			config := corev1.Volume{
				Name: "krb5.conf",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: kfb.Spec.Net.GSSAPI.Config.SecretKeyRef.Name,
						Items: []corev1.KeyToPath{
							{
								Key:  kfb.Spec.Net.GSSAPI.Config.SecretKeyRef.Key,
								Path: "krb5.conf",
							},
						},
					},
				},
			}
			spec.Volumes = append(spec.Volumes, config)
		}
	}

	for i := range spec.InitContainers {

		spec.InitContainers[i].Env = append(spec.InitContainers[i].Env, corev1.EnvVar{
			Name:  "KAFKA_BOOTSTRAP_SERVERS",
			Value: strings.Join(kfb.Spec.BootstrapServers, ","),
		})
		if kfb.Spec.Net.SASL.Enable {
			spec.InitContainers[i].Env = append(spec.InitContainers[i].Env, corev1.EnvVar{
				Name:  "KAFKA_NET_SASL_ENABLE",
				Value: "true",
			}, corev1.EnvVar{
				Name: "KAFKA_NET_SASL_USER",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: kfb.Spec.Net.SASL.User.SecretKeyRef,
				},
			}, corev1.EnvVar{
				Name: "KAFKA_NET_SASL_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: kfb.Spec.Net.SASL.Password.SecretKeyRef,
				},
			}, corev1.EnvVar{
				Name: "KAFKA_NET_SASL_TYPE",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: kfb.Spec.Net.SASL.Type.SecretKeyRef,
				},
			})
		}

		if kfb.Spec.Net.GSSAPI.Enable {

			if kfb.Spec.Net.GSSAPI.KeyTab.SecretKeyRef != nil {
				keytab := corev1.VolumeMount{
					Name:      "krb5.keytab",
					MountPath: "/etc/",
				}
				spec.InitContainers[i].VolumeMounts = append(spec.InitContainers[i].VolumeMounts, keytab)

				spec.InitContainers[i].Env = append(spec.InitContainers[i].Env, corev1.EnvVar{
					Name:  "KAFKA_NET_SASL_KERBEROS_KEYTAB_FILE",
					Value: path.Join(keytab.MountPath, keytab.Name),
				})
			}

			if kfb.Spec.Net.GSSAPI.Config.SecretKeyRef != nil {
				config := corev1.VolumeMount{
					Name:      "krb5.conf",
					MountPath: "/etc/",
				}
				spec.InitContainers[i].VolumeMounts = append(spec.InitContainers[i].VolumeMounts, config)

				spec.InitContainers[i].Env = append(spec.InitContainers[i].Env, corev1.EnvVar{
					Name:  "KAFKA_NET_SASL_KERBEROS_CONFIG_FILE",
					Value: path.Join(config.MountPath, config.Name),
				})
			}

			spec.InitContainers[i].Env = append(spec.InitContainers[i].Env, corev1.EnvVar{
				Name:  "KAFKA_NET_SASL_KERBEROS_ENABLE",
				Value: "true",
			}, corev1.EnvVar{
				Name: "KAFKA_NET_SASL_KERBEROS_PRINCIPAL",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: kfb.Spec.Net.GSSAPI.Principal.SecretKeyRef,
				},
			}, corev1.EnvVar{
				Name: "KAFKA_NET_SASL_KERBEROS_SERVICE",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: kfb.Spec.Net.GSSAPI.Service.SecretKeyRef,
				},
			}, corev1.EnvVar{
				Name: "KAFKA_NET_SASL_KERBEROS_REALM",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: kfb.Spec.Net.GSSAPI.Realm.SecretKeyRef,
				},
			}, corev1.EnvVar{
				Name: "KAFKA_NET_SASL_KERBEROS_USERNAME",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: kfb.Spec.Net.GSSAPI.Username.SecretKeyRef,
				},
			}, corev1.EnvVar{
				Name: "KAFKA_NET_SASL_KERBEROS_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: kfb.Spec.Net.GSSAPI.Password.SecretKeyRef,
				},
			})
		}

		if kfb.Spec.Net.TLS.Enable {
			spec.InitContainers[i].Env = append(spec.InitContainers[i].Env, corev1.EnvVar{
				Name:  "KAFKA_NET_TLS_ENABLE",
				Value: "true",
			}, corev1.EnvVar{
				Name: "KAFKA_NET_TLS_CERT",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: kfb.Spec.Net.TLS.Cert.SecretKeyRef,
				},
			}, corev1.EnvVar{
				Name: "KAFKA_NET_TLS_KEY",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: kfb.Spec.Net.TLS.Key.SecretKeyRef,
				},
			}, corev1.EnvVar{
				Name: "KAFKA_NET_TLS_CA_CERT",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: kfb.Spec.Net.TLS.CACert.SecretKeyRef,
				},
			})
		}
	}

	for i := range spec.Containers {
		spec.Containers[i].Env = append(spec.Containers[i].Env, corev1.EnvVar{
			Name:  "KAFKA_BOOTSTRAP_SERVERS",
			Value: strings.Join(kfb.Spec.BootstrapServers, ","),
		})

		if kfb.Spec.Net.SASL.Enable {
			spec.Containers[i].Env = append(spec.Containers[i].Env, corev1.EnvVar{
				Name:  "KAFKA_NET_SASL_ENABLE",
				Value: "true",
			}, corev1.EnvVar{
				Name: "KAFKA_NET_SASL_USER",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: kfb.Spec.Net.SASL.User.SecretKeyRef,
				},
			}, corev1.EnvVar{
				Name: "KAFKA_NET_SASL_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: kfb.Spec.Net.SASL.Password.SecretKeyRef,
				},
			}, corev1.EnvVar{
				Name: "KAFKA_NET_SASL_TYPE",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: kfb.Spec.Net.SASL.Type.SecretKeyRef,
				},
			})
		}

		if kfb.Spec.Net.GSSAPI.Enable {

			if kfb.Spec.Net.GSSAPI.KeyTab.SecretKeyRef != nil {
				keytab := corev1.VolumeMount{
					Name:      "krb5.keytab",
					MountPath: "/etc/",
				}
				spec.Containers[i].VolumeMounts = append(spec.Containers[i].VolumeMounts, keytab)

				spec.Containers[i].Env = append(spec.Containers[i].Env, corev1.EnvVar{
					Name:  "KAFKA_NET_SASL_KERBEROS_KEYTAB_FILE",
					Value: path.Join(keytab.MountPath, keytab.Name),
				})
			}

			if kfb.Spec.Net.GSSAPI.Config.SecretKeyRef != nil {
				config := corev1.VolumeMount{
					Name:      "krb5.conf",
					MountPath: "/etc/",
				}
				spec.Containers[i].VolumeMounts = append(spec.Containers[i].VolumeMounts, config)

				spec.Containers[i].Env = append(spec.Containers[i].Env, corev1.EnvVar{
					Name:  "KAFKA_NET_SASL_KERBEROS_CONFIG_FILE",
					Value: path.Join(config.MountPath, config.Name),
				})
			}

			spec.Containers[i].Env = append(spec.Containers[i].Env, corev1.EnvVar{
				Name:  "KAFKA_NET_SASL_KERBEROS_ENABLE",
				Value: "true",
			}, corev1.EnvVar{
				Name: "KAFKA_NET_SASL_KERBEROS_PRINCIPAL",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: kfb.Spec.Net.GSSAPI.Principal.SecretKeyRef,
				},
			}, corev1.EnvVar{
				Name: "KAFKA_NET_SASL_KERBEROS_SERVICE",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: kfb.Spec.Net.GSSAPI.Service.SecretKeyRef,
				},
			}, corev1.EnvVar{
				Name: "KAFKA_NET_SASL_KERBEROS_REALM",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: kfb.Spec.Net.GSSAPI.Realm.SecretKeyRef,
				},
			}, corev1.EnvVar{
				Name: "KAFKA_NET_SASL_KERBEROS_USERNAME",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: kfb.Spec.Net.GSSAPI.Username.SecretKeyRef,
				},
			}, corev1.EnvVar{
				Name: "KAFKA_NET_SASL_KERBEROS_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: kfb.Spec.Net.GSSAPI.Password.SecretKeyRef,
				},
			})
		}

		if kfb.Spec.Net.TLS.Enable {
			spec.Containers[i].Env = append(spec.Containers[i].Env, corev1.EnvVar{
				Name:  "KAFKA_NET_TLS_ENABLE",
				Value: "true",
			}, corev1.EnvVar{
				Name: "KAFKA_NET_TLS_CERT",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: kfb.Spec.Net.TLS.Cert.SecretKeyRef,
				},
			}, corev1.EnvVar{
				Name: "KAFKA_NET_TLS_KEY",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: kfb.Spec.Net.TLS.Key.SecretKeyRef,
				},
			}, corev1.EnvVar{
				Name: "KAFKA_NET_TLS_CA_CERT",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: kfb.Spec.Net.TLS.CACert.SecretKeyRef,
				},
			})
		}
	}
}

func (kfb *KafkaBinding) Undo(ctx context.Context, ps *duckv1.WithPod) {
	spec := ps.Spec.Template.Spec

	for i, c := range spec.InitContainers {
		if len(c.Env) == 0 {
			continue
		}
		env := make([]corev1.EnvVar, 0, len(spec.InitContainers[i].Env))
		for j, ev := range c.Env {
			switch ev.Name {
			case "KAFKA_NET_TLS_ENABLE", "KAFKA_NET_TLS_CERT", "KAFKA_NET_TLS_KEY", "KAFKA_NET_TLS_CA_CERT",
				"KAFKA_NET_SASL_ENABLE", "KAFKA_NET_SASL_USER", "KAFKA_NET_SASL_PASSWORD",
				"KAFKA_NET_SASL_KERBEROS_ENABLE", "KAFKA_NET_SASL_KERBEROS_KEYTAB_FILE", "KAFKA_NET_SASL_KERBEROS_CONFIG_FILE",
				"KAFKA_NET_SASL_KERBEROS_PRINCIPAL", "KAFKA_NET_SASL_KERBEROS_SERVICE", "KAFKA_NET_SASL_KERBEROS_REALM",
				"KAFKA_NET_SASL_KERBEROS_USERNAME", "KAFKA_NET_SASL_KERBEROS_PASSWORD",
				"KAFKA_NET_SASL_TYPE", "KAFKA_BOOTSTRAP_SERVERS":

				continue
			default:
				env = append(env, spec.InitContainers[i].Env[j])
			}
		}
		spec.InitContainers[i].Env = env
	}

	for i, c := range spec.Containers {
		if len(c.Env) == 0 {
			continue
		}
		env := make([]corev1.EnvVar, 0, len(spec.Containers[i].Env))
		for j, ev := range c.Env {
			switch ev.Name {
			case "KAFKA_NET_TLS_ENABLE", "KAFKA_NET_TLS_CERT", "KAFKA_NET_TLS_KEY", "KAFKA_NET_TLS_CA_CERT",
				"KAFKA_NET_SASL_ENABLE", "KAFKA_NET_SASL_USER", "KAFKA_NET_SASL_PASSWORD",
				"KAFKA_NET_SASL_KERBEROS_ENABLE", "KAFKA_NET_SASL_KERBEROS_KEYTAB_FILE", "KAFKA_NET_SASL_KERBEROS_CONFIG_FILE",
				"KAFKA_NET_SASL_KERBEROS_PRINCIPAL", "KAFKA_NET_SASL_KERBEROS_SERVICE", "KAFKA_NET_SASL_KERBEROS_REALM",
				"KAFKA_NET_SASL_KERBEROS_USERNAME", "KAFKA_NET_SASL_KERBEROS_PASSWORD",
				"KAFKA_NET_SASL_TYPE", "KAFKA_BOOTSTRAP_SERVERS":
				continue
			default:
				env = append(env, spec.Containers[i].Env[j])
			}
		}
		spec.Containers[i].Env = env
	}
}
