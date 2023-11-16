/*
Copyright 2023.

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
	"fmt"
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	hogv1 "github.com/ImmortalHedgehogs/HogEye/api/v1"
)

// HogEyeReconciler reconciles a HogEye object
type HogEyeReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=hog.immortal.hedgehogs,resources=hogeyes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=hog.immortal.hedgehogs,resources=hogeyes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=hog.immortal.hedgehogs,resources=hogeyes/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete

//+kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the HogEye object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.3/pkg/reconcile
func (r *HogEyeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	hogeye := &hogv1.HogEye{}

	// Retrieve our CR
	if err := r.Get(ctx, req.NamespacedName, hogeye); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	hogeyeFinalizer := "hog.immortal.hedgehogs/finalizer"

	// examine DeletionTimestamp to determine if object is under deletion
	if hogeye.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !controllerutil.ContainsFinalizer(hogeye, hogeyeFinalizer) {
			controllerutil.AddFinalizer(hogeye, hogeyeFinalizer)
			if err := r.Update(ctx, hogeye); err != nil {
				return ctrl.Result{}, err
			}
		}

		// Create a deployment key
		deploymentKey := client.ObjectKey{
			Name:      hogeye.Name + "-deployment",
			Namespace: hogeye.Namespace,
		}

		// Check if the Deployment exists before attempting to create it
		var existingDeployment appsv1.Deployment
		if err := r.Get(ctx, deploymentKey, &existingDeployment); err != nil {
			if client.IgnoreNotFound(err) == nil {
				// The Deployment does not exist, so create it
				if err := r.createExternalResources(ctx, *hogeye); err != nil {
					log.Error(err, "Unable to create external resources")
					hogeye.Status.Status = "Error"
					r.Status().Update(ctx, hogeye)
					return ctrl.Result{}, err
				} else {
					hogeye.Status.Status = "Watching"
				}

				if err := r.Status().Update(ctx, hogeye); err != nil {
					log.Error(err, "unable to update HogEye status 0")
					return ctrl.Result{}, err
				}
				log.Info("Created")

			} else {
				// handle error
				log.Error(err, "unable to find deployment")
			}
		} else {
			// UPDATING DEPLOYMENT IF NEEDED
			deployment, err := r.createDeployment(*hogeye)
			if err != nil {
				log.Error(err, "Unable to create updated deployment")
				hogeye.Status.Status = "Error"
				r.Status().Update(ctx, hogeye)
				return ctrl.Result{}, err
			}

			if !reflect.DeepEqual(deployment.Spec.Template, existingDeployment.Spec.Template) {
				existingDeployment.Spec = deployment.Spec
				if err := r.Update(ctx, &existingDeployment); err != nil {
					// if fail to delete the external dependency here, return with error
					// so that it can be retried
					return ctrl.Result{}, err
				}
				log.Info("Updated")
			}

			// VERIFY OUR OTHER CHILD RESOURCES STILL EXIST
			var existingServiceAccount corev1.ServiceAccount

			serviceAccountKey := client.ObjectKey{
				Name:      hogeye.Name + "-service-account",
				Namespace: hogeye.Namespace,
			}

			if err := r.Get(ctx, serviceAccountKey, &existingServiceAccount); err != nil {
				if client.IgnoreNotFound(err) == nil {
					serviceAccount, err := r.createServiceAccount(*hogeye)
					if err != nil {
						log.Error(err, "Failed to create service account")
						return ctrl.Result{}, err
					} else {
						if err := controllerutil.SetControllerReference(hogeye, &serviceAccount, r.Scheme); err != nil {
							return ctrl.Result{}, err
						}
					}

					if err := r.Create(ctx, &serviceAccount); err != nil {
						log.Error(err, "Unable to create Service Account")
						return ctrl.Result{}, err
					}

					deployment, err := r.createDeployment(*hogeye)
					if err != nil {
						log.Error(err, "Unable to create updated deployment")
						hogeye.Status.Status = "Error"
						r.Status().Update(ctx, hogeye)
						return ctrl.Result{}, err
					}

					existingDeployment.Spec = deployment.Spec
					if err := r.Update(ctx, &existingDeployment); err != nil {
						// if fail to delete the external dependency here, return with error
						// so that it can be retried
						return ctrl.Result{}, err
					}
				}
			}

			var existingRole rbacv1.Role

			roleKey := client.ObjectKey{
				Name:      hogeye.Name + "-pod-role",
				Namespace: hogeye.Namespace,
			}

			if err := r.Get(ctx, roleKey, &existingRole); err != nil {
				if client.IgnoreNotFound(err) == nil {
					role, err := r.createRole(*hogeye)
					if err != nil {
						log.Error(err, "Failed to create role")
						return ctrl.Result{}, err
					} else {
						if err := controllerutil.SetControllerReference(hogeye, &role, r.Scheme); err != nil {
							return ctrl.Result{}, err
						}
					}

					if err := r.Create(ctx, &role); err != nil {
						log.Error(err, "Unable to create Role")
						return ctrl.Result{}, err
					}
				}
			}

			var existingRoleBinding rbacv1.RoleBinding

			roleBindingKey := client.ObjectKey{
				Name:      hogeye.Name + "-pod-role-binding",
				Namespace: hogeye.Namespace,
			}

			if err := r.Get(ctx, roleBindingKey, &existingRoleBinding); err != nil {
				if client.IgnoreNotFound(err) == nil {
					roleBinding, err := r.createRoleBinding(*hogeye)
					if err != nil {
						log.Error(err, "Failed to create rolebinding")
						return ctrl.Result{}, err
					} else {
						if err := controllerutil.SetControllerReference(hogeye, &roleBinding, r.Scheme); err != nil {
							return ctrl.Result{}, err
						}
					}

					if err := r.Create(ctx, &roleBinding); err != nil {
						log.Error(err, "Unable to create RoleBinding")
						return ctrl.Result{}, err
					}
				}
			}

			if err := r.Status().Update(ctx, hogeye); err != nil {
				log.Error(err, "unable to update HogEye status 0")
				return ctrl.Result{}, err
			}
		}
	} else {
		hogeye.Status.Status = "Terminating"
		if err := r.Status().Update(ctx, hogeye); err != nil {
			log.Error(err, "unable to update HogEye status 3")
			return ctrl.Result{}, err
		}
		// The object is being deleted
		if controllerutil.ContainsFinalizer(hogeye, hogeyeFinalizer) {
			// our finalizer is present, so lets handle any external dependency
			if err := r.deleteExternalResources(ctx, *hogeye); err != nil {
				// if fail to delete the external dependency here, return with error
				// so that it can be retried
				return ctrl.Result{}, err
			}

			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(hogeye, hogeyeFinalizer)
			if err := r.Update(ctx, hogeye); err != nil {
				return ctrl.Result{}, err
			}
		}

		// Stop reconciliation as the item is being deleted
		log.Info("Deleted")
	}

	return ctrl.Result{}, nil
}

func (r *HogEyeReconciler) createServiceAccount(hogeye hogv1.HogEye) (corev1.ServiceAccount, error) {
	automount := true
	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hogeye.Name + "-service-account",
			Namespace: hogeye.Namespace,
		},
		AutomountServiceAccountToken: &automount,
	}

	return *serviceAccount, nil
}

func (r *HogEyeReconciler) createRole(hogeye hogv1.HogEye) (rbacv1.Role, error) {
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hogeye.Name + "-pod-role",
			Namespace: hogeye.Namespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				Verbs:     []string{"get", "list"},
				APIGroups: []string{""},
				Resources: []string{"pods"},
			},
		},
	}

	return *role, nil
}

func (r *HogEyeReconciler) createRoleBinding(hogeye hogv1.HogEye) (rbacv1.RoleBinding, error) {
	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hogeye.Name + "-pod-role-binding",
			Namespace: hogeye.Namespace,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      hogeye.Name + "-service-account",
				Namespace: hogeye.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "",
			Kind:     "Role",
			Name:     hogeye.Name + "-pod-role",
		},
	}

	return *roleBinding, nil
}

func (r *HogEyeReconciler) createDeployment(hogeye hogv1.HogEye) (appsv1.Deployment, error) {
	replicaCount := int32(1)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hogeye.Name + "-deployment",
			Namespace: hogeye.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicaCount,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "demo",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "demo",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "hogeye",
							Image:           "traviskroll/hogeye:v1",
							ImagePullPolicy: "Always",
							EnvFrom: []corev1.EnvFromSource{
								{
									SecretRef: &corev1.SecretEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: hogeye.Spec.AppTokenSecret,
										},
									},
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "SLACKCHANNEL",
									Value: hogeye.Spec.SlackChannels,
								},
								{
									Name:  "RESOURCE",
									Value: hogeye.Spec.QueryResources,
								},
								{
									Name:  "NAMESPACE",
									Value: hogeye.Spec.QueryNamespace,
								},
								{
									Name:  "AGETHRESHOLD",
									Value: fmt.Sprint(hogeye.Spec.AgeThreshold),
								},
								{
									Name:  "QUERYTIME",
									Value: hogeye.Spec.QueryTime,
								},
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									Protocol:      corev1.ProtocolTCP,
									ContainerPort: 3000,
								},
							},
						},
					},
					ServiceAccountName: hogeye.Name + "-service-account",
				},
			},
		},
	}

	if err := controllerutil.SetControllerReference(&hogeye, deployment, r.Scheme); err != nil {
		return *deployment, err
	}

	return *deployment, nil
}

func (r *HogEyeReconciler) createExternalResources(ctx context.Context, hogeye hogv1.HogEye) error {
	log := log.FromContext(ctx)

	var allErrs []error

	//create service account, role, and role binding for the deployment pods
	serviceAccount, err := r.createServiceAccount(hogeye)
	if err != nil {
		log.Error(err, "Failed to create service account")
		allErrs = append(allErrs, err)
	} else {
		if err := controllerutil.SetControllerReference(&hogeye, &serviceAccount, r.Scheme); err != nil {
			allErrs = append(allErrs, err)
		}
	}

	role, err := r.createRole(hogeye)
	if err != nil {
		log.Error(err, "Failed to create role")
		allErrs = append(allErrs, err)
	} else {
		if err := controllerutil.SetControllerReference(&hogeye, &role, r.Scheme); err != nil {
			allErrs = append(allErrs, err)
		}
	}

	roleBinding, err := r.createRoleBinding(hogeye)
	if err != nil {
		log.Error(err, "Failed to create role binding")
		allErrs = append(allErrs, err)
	} else {
		if err := controllerutil.SetControllerReference(&hogeye, &roleBinding, r.Scheme); err != nil {
			allErrs = append(allErrs, err)
		}
	}

	// apply service account, role, and role binding
	if err := r.Create(ctx, &serviceAccount); err != nil {
		log.Error(err, "Unable to create Service Account")
		allErrs = append(allErrs, err)
	}

	if err := r.Create(ctx, &role); err != nil {
		log.Error(err, "Unable to create role")
		allErrs = append(allErrs, err)
	}

	if err := r.Create(ctx, &roleBinding); err != nil {
		log.Error(err, "Unable to create role")
		allErrs = append(allErrs, err)
	}

	deployment, err := r.createDeployment(hogeye)
	if err != nil {
		log.Error(err, "failed to create Deployment Spec")
		allErrs = append(allErrs, err)
	} /*else {
		if err := controllerutil.SetControllerReference(&hogeye, &deployment, r.Scheme); err != nil {
			allErrs = append(allErrs, err)
		}
	}*/
	if err := r.Create(ctx, &deployment); err != nil {
		log.Error(err, "unable to create Deployment")
		allErrs = append(allErrs, err)
	}

	return nil
}

func (r *HogEyeReconciler) deleteExternalResources(ctx context.Context, hogeye hogv1.HogEye) error {
	log := log.FromContext(ctx)

	// DELETE DEPLOYMENT
	deploymentKey := client.ObjectKey{
		Name:      hogeye.Name + "-deployment",
		Namespace: hogeye.Namespace,
	}

	// Check if the Job exists before attempting to delete it
	var existingDeployment appsv1.Deployment
	if err := r.Get(ctx, deploymentKey, &existingDeployment); err != nil {
		return err
	}

	if err := r.Delete(ctx, &existingDeployment); err != nil {
		log.Error(err, "unable to delete Deployment")
	}

	// DELETE ROLE BINDING
	roleBindingKey := client.ObjectKey{
		Name:      hogeye.Name + "-pod-role-binding",
		Namespace: hogeye.Namespace,
	}

	var existingRoleBinding rbacv1.RoleBinding
	if err := r.Get(ctx, roleBindingKey, &existingRoleBinding); err != nil {
		return err
	}

	if err := r.Delete(ctx, &existingRoleBinding); err != nil {
		log.Error(err, "Unable to delete role binding")
	}

	// DELETE ROLE
	roleKey := client.ObjectKey{
		Name:      hogeye.Name + "-pod-role",
		Namespace: hogeye.Namespace,
	}

	var existingRole rbacv1.Role
	if err := r.Get(ctx, roleKey, &existingRole); err != nil {
		return err
	}

	if err := r.Delete(ctx, &existingRole); err != nil {
		log.Error(err, "Unable to delete role")
	}

	// DELETE SERVICE ACCOUNT
	serviceAccountKey := client.ObjectKey{
		Name:      hogeye.Name + "-service-account",
		Namespace: hogeye.Namespace,
	}

	var existingServiceAccount corev1.ServiceAccount
	if err := r.Get(ctx, serviceAccountKey, &existingServiceAccount); err != nil {
		return err
	}

	if err := r.Delete(ctx, &existingServiceAccount); err != nil {
		log.Error(err, "Unable to delete Service Account")
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *HogEyeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hogv1.HogEye{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&rbacv1.Role{}).
		Owns(&rbacv1.RoleBinding{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}
