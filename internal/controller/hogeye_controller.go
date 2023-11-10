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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
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

	// MAY CAUSE PROBLEMS
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

		// Check if the Job exists before attempting to create it
		var existingDeployment appsv1.Deployment
		if err := r.Get(ctx, deploymentKey, &existingDeployment); err != nil {
			if client.IgnoreNotFound(err) == nil {
				// The Job does not exist, so create it
				deployment, err := r.createDeployment(*hogeye)
				if err != nil {
					log.Error(err, "failed to create Deployment Spec")
					return ctrl.Result{}, client.IgnoreNotFound(err)
				}

				if err := r.Create(ctx, &deployment); err != nil {
					log.Error(err, "unable to create Deployment")
				}
			} else {
				// handle error
				log.Error(err, "unable to find deployment")
			}
		} else {
			// We found the job, update it (by update we mean tear down old and make a new one...)

			if err := r.deleteExternalResources(ctx, *hogeye); err != nil {
				// if fail to delete the external dependency here, return with error
				// so that it can be retried
				return ctrl.Result{}, err
			}

			// The Job does not exist, so create it
			deployment, err := r.createDeployment(*hogeye)
			if err != nil {
				log.Error(err, "failed to create Deployment Spec")
				return ctrl.Result{}, client.IgnoreNotFound(err)
			}
			if err := r.Create(ctx, &deployment); err != nil {
				log.Error(err, "unable to create Deployment")
			}

			log.Info("Updated")
		}

	} else {
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
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

func (r *HogEyeReconciler) createDeployment(hogeye hogv1.HogEye) (appsv1.Deployment, error) {
	// hogeye.Spec.

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
							Name:  "web",
							Image: "nginx:1.12",
							Env: []corev1.EnvVar{
								{
									Name:  "APPSECRET",
									Value: hogeye.Spec.AppTokenSecret,
								},
								{
									Name:  "BOTSECRET",
									Value: hogeye.Spec.BotTokenSecret,
								},
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
									ContainerPort: 80,
								},
							},
						},
					},
				},
			},
		},
	}

	return *deployment, nil
}

func (r *HogEyeReconciler) deleteExternalResources(ctx context.Context, hogeye hogv1.HogEye) error {
	log := log.FromContext(ctx)

	// Create a deployment key
	deploymentKey := client.ObjectKey{
		Name:      hogeye.Name + "-deployment",
		Namespace: hogeye.Namespace,
	}

	// Check if the Job exists before attempting to delete it
	var existingDeployment appsv1.Deployment
	if err := r.Get(ctx, deploymentKey, &existingDeployment); err != nil {
		if client.IgnoreNotFound(err) == nil {
			// The Job exists, so delete it
			return nil
		} else {
			// The Job does not exist, or there was an error
			return err
		}
	}

	if err := r.Delete(ctx, &existingDeployment); err != nil {
		log.Error(err, "unable to delete Deployment")
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *HogEyeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hogv1.HogEye{}).
		Complete(r)
}
