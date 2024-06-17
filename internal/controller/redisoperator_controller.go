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

		configMap := newConfigMap(req.Name, req.Namespace, redisVersion, exporterVersion, teamName)
		if err := r.Create(ctx, configMap); err != nil {
			return ctrl.Result{}, err
		}

		pvcData := newPVC(req.Name, req.Namespace, teamName)
		if err := r.Create(ctx, pvcData); err != nil {
			return ctrl.Result{}, err
		}

		redisDeployment := newRedisDeployment(req.Name, req.Namespace, redisVersion, teamName)
		if err := r.Create(ctx, redisDeployment); err != nil {
			return ctrl.Result{}, err
		}

		exporterDeployment := newRedisExporterDeployment(req.Name, req.Namespace, exporterVersion, teamName)
		if err := r.Create(ctx, exporterDeployment); err != nil {
			return ctrl.Result{}, err
		}

		redisSvc := newRedisService("redis-svc", req.Namespace, teamName)
		if err := r.Create(ctx, redisSvc); err != nil {
			return ctrl.Result{}, err
		}
	} else {
		// The RedisOperator is being deleted, perform cleanup tasks
		// Delete the ConfigMap
		configMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "redis-config",
				Namespace: instance.Namespace,
			},
		}
		if err := r.Delete(ctx, configMap); client.IgnoreNotFound(err) != nil {
			return ctrl.Result{}, err
		}

		// Delete the PVC
		pvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "redis-data",
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
				Name:      "redis-svc",
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

func newConfigMap(name, namespace string, redisVersion string, exporterVersion string, teamName string) *corev1.ConfigMap {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "redis-config",
			Namespace: namespace,
		},
		Data: map[string]string{
			"redis.conf": `
				save 60 1
				save 300 10
				save 900 100
				save 86400 1
				dir /data
				dbfilename dump.rdb
			`,
		},
	}
	return cm
}

func newPVC(name, namespace string, teamName string) *corev1.PersistentVolumeClaim {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "redis-data",
			Namespace: namespace,
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

func newRedisDeployment(name, namespace string, redisVersion string, teamName string) *appsv1.Deployment {
	labels := map[string]string{
		"app":  "redis",
		"team": teamName,
	}
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
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
							Command: []string{"redis-server", "/usr/local/etc/redis/redis.conf"},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "redis-storage",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "redis-data",
								},
							},
						},
						{
							Name: "redis-config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "redis-config",
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

func newRedisExporterDeployment(name, namespace string, exporterVersion string, teamName string) *appsv1.Deployment {
	labels := map[string]string{
		"app":  "redis-exporter",
		"team": teamName,
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
							Env: []corev1.EnvVar{
								{
									Name:  "REDIS_ADDR",
									Value: "redis://redis-svc:6379",
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

func newRedisService(name, namespace string, teamName string) *corev1.Service {
	labels := map[string]string{
		"app":  "redis",
		"team": teamName,
	}
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
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
