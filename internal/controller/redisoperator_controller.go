/*
Copyright 2024 MatanMagen.

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

package controller

import (
	"context"
	"math/rand"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apiv1alpha1 "github.com/MatanMagen/redis-operator.git/api/v1alpha1"
)

const redisOperatorFinalizer = "finalizer.redisoperators.api.core.matanmagen.io"

// RedisOperatorReconciler reconciles a RedisOperator object
type RedisOperatorReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=api.core.matanmagen.io,resources=redisoperators,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=api.core.matanmagen.io,resources=redisoperators/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=api.core.matanmagen.io,resources=redisoperators/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the RedisOperator object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.18.2/pkg/reconcile
func (r *RedisOperatorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	instance := &apiv1alpha1.RedisOperator{}
	err := r.Client.Get(context.TODO(), req.NamespacedName, instance)
	if err != nil {
		return ctrl.Result{}, err
	}

	if instance.DeletionTimestamp.IsZero() {
		// The RedisOperator is not being deleted, proceed with normal reconcile logic
		if !containsString(instance.Finalizers, redisOperatorFinalizer) {
			instance.Finalizers = append(instance.Finalizers, redisOperatorFinalizer)
			if err := r.Update(context.TODO(), instance); err != nil {
				return ctrl.Result{}, err
			}
		}

		// Now you can access the Spec fields
		redisVersion := instance.Spec.RedisVersion
		exporterVersion := instance.Spec.ExporterVersion
		teamName := instance.Spec.TeamName
		env := instance.Spec.Env

		password := generateRandomPassword(10) // generate a 10-character random password

		redisPassword := newPassword(password, req.Name, req.Namespace, teamName, env)
		if err := r.Create(ctx, redisPassword); err != nil {
			return ctrl.Result{}, err
		}

		configMap := newConfigMap(req.Name, req.Namespace, teamName, env)
		if err := r.Create(ctx, configMap); err != nil {
			return ctrl.Result{}, err
		}

		pvcData := newPVC(req.Name, req.Namespace, teamName, env)
		if err := r.Create(ctx, pvcData); err != nil {
			return ctrl.Result{}, err
		}

		redisDeployment := newRedisDeployment(req.Name, req.Namespace, redisVersion, teamName, env)
		if err := r.Create(ctx, redisDeployment); err != nil {
			return ctrl.Result{}, err
		}

		exporterDeployment := newRedisExporterDeployment(req.Name, req.Namespace, exporterVersion, teamName, env)
		if err := r.Create(ctx, exporterDeployment); err != nil {
			return ctrl.Result{}, err
		}

		redisSvc := newRedisService(req.Name, req.Namespace, teamName, env)
		if err := r.Create(ctx, redisSvc); err != nil {
			return ctrl.Result{}, err
		}
	} else {
		// The RedisOperator is being deleted, perform cleanup tasks

		// Delete the Redis Password Secret
		redisPassword := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "redis-password-" + req.Name,
				Namespace: instance.Namespace,
			},
		}
		if err := r.Delete(ctx, redisPassword); client.IgnoreNotFound(err) != nil {
			return ctrl.Result{}, err
		}

		// Delete the ConfigMap
		configMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "redis-config-" + req.Name,
				Namespace: instance.Namespace,
			},
		}
		if err := r.Delete(ctx, configMap); client.IgnoreNotFound(err) != nil {
			return ctrl.Result{}, err
		}

		// Delete the PVC
		pvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "redis-data-" + req.Name,
				Namespace: instance.Namespace,
			},
		}
		if err := r.Delete(ctx, pvc); client.IgnoreNotFound(err) != nil {
			return ctrl.Result{}, err
		}

		// Delete the Redis Deployment
		redisDeployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      instance.Name,
				Namespace: instance.Namespace,
			},
		}
		if err := r.Delete(ctx, redisDeployment); client.IgnoreNotFound(err) != nil {
			return ctrl.Result{}, err
		}

		// Delete the Redis Exporter Deployment
		exporterDeployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      instance.Name + "-exporter",
				Namespace: instance.Namespace,
			},
		}
		if err := r.Delete(ctx, exporterDeployment); client.IgnoreNotFound(err) != nil {
			return ctrl.Result{}, err
		}

		// Delete the Redis Service
		redisSvc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "redis-svc-" + instance.Name,
				Namespace: instance.Namespace,
			},
		}
		if err := r.Delete(ctx, redisSvc); client.IgnoreNotFound(err) != nil {
			return ctrl.Result{}, err
		}

		instance.Finalizers = removeString(instance.Finalizers, redisOperatorFinalizer)
		if err := r.Update(context.TODO(), instance); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// containsString checks if a list contains a string
func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// removeString removes a string from a list
func removeString(list []string, s string) (result []string) {
	for _, v := range list {
		if v != s {
			result = append(result, v)
		}
	}
	return
}

func generateRandomPassword(length int) string {
	source := rand.NewSource(time.Now().UnixNano())
	random := rand.New(source)
	chars := []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZ" +
		"abcdefghijklmnopqrstuvwxyz" +
		"0123456789")
	var b strings.Builder
	for i := 0; i < length; i++ {
		b.WriteRune(chars[random.Intn(len(chars))])
	}
	return b.String()
}

func newPassword(pass, name, namespace string, teamName string, env string) *corev1.Secret {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "redis-password-" + name,
			Namespace: namespace,
			Labels: map[string]string{
				"project": teamName,
				"env":     env,
			},
		},
		StringData: map[string]string{
			"password": pass,
		},
	}
	return secret
}

func newConfigMap(name, namespace string, teamName string, env string) *corev1.ConfigMap {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "redis-config-" + name,
			Namespace: namespace,
			Labels: map[string]string{
				"project": teamName,
				"env":     env,
			},
		},
		Data: map[string]string{
			"redis.conf": `
				save 1 1
				dir /data
				dbfilename dump.rdb
			`,
		},
	}
	return cm
}

func newPVC(name, namespace string, teamName string, env string) *corev1.PersistentVolumeClaim {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "redis-data-" + name,
			Namespace: namespace,
			Labels: map[string]string{
				"project": teamName,
				"env":     env,
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("10Gi"),
				},
			},
		},
	}
	return pvc
}
func newRedisDeployment(name, namespace string, redisVersion string, teamName string, env string) *appsv1.Deployment {
	labels := map[string]string{
		"app":     "redis",
		"project": teamName,
		"env":     env,
	}
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: func(i int32) *int32 { return &i }(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "redis",
							Image: "redis",
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 6379,
								},
							},
							Env: []corev1.EnvVar{
								{
									Name: "REDIS_PASSWORD",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "redis-password-" + name,
											},
											Key: "password",
										},
									},
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("100Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("200m"),
									corev1.ResourceMemory: resource.MustParse("200Mi"),
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "redis-storage",
									MountPath: "/data",
								},
								{
									Name:      "redis-config",
									MountPath: "/usr/local/etc/redis/redis.conf",
									SubPath:   "redis.conf",
								},
							},
							Command: []string{"redis-server", "/usr/local/etc/redis/redis.conf", "--requirepass", "$(REDIS_PASSWORD)"},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "redis-storage",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "redis-data-" + name,
								},
							},
						},
						{
							Name: "redis-config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "redis-config-" + name,
									},
								},
							},
						},
					},
				},
			},
		},
	}
	return dep
}

func newRedisExporterDeployment(name, namespace string, exporterVersion string, teamName string, env string) *appsv1.Deployment {
	labels := map[string]string{
		"app":     "redis-exporter",
		"project": teamName,
		"env":     env,
	}
	annotations := map[string]string{
		"prometheus.io/scrape": "true",
		"prometheus.io/port":   "9121",
		"prometheus.io/path":   "/metrics",
	}
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name + "-exporter",
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: func(i int32) *int32 { return &i }(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: annotations,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "redis-exporter",
							Image: "oliver006/redis_exporter:" + exporterVersion,
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 9121,
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("100Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("200m"),
									corev1.ResourceMemory: resource.MustParse("200Mi"),
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "REDIS_ADDR",
									Value: "redis://redis-svc:6379",
								},
								// secret for password
								{
									Name: "REDIS_PASSWORD",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "redis-password-" + name,
											},
											Key: "password",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	return dep
}

func newRedisService(name, namespace string, teamName string, env string) *corev1.Service {
	labels := map[string]string{
		"app":     "redis",
		"project": teamName,
		"env":     env,
	}
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "redis-svc-" + name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports: []corev1.ServicePort{
				{
					Port:       6379,
					TargetPort: intstr.FromInt(6379),
				},
			},
		},
	}
	return svc
}

// SetupWithManager sets up the controller with the Manager.
func (r *RedisOperatorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&apiv1alpha1.RedisOperator{}).
		Complete(r)
}
