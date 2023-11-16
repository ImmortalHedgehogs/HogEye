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

package v1

import (
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var hogeyelog = logf.Log.WithName("hogeye-resource")

func (r *HogEye) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-hog-immortal-hedgehogs-v1-hogeye,mutating=true,failurePolicy=fail,sideEffects=None,groups=hog.immortal.hedgehogs,resources=hogeyes,verbs=create;update,versions=v1,name=mhogeye.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &HogEye{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *HogEye) Default() {
	hogeyelog.Info("default", "name", r.Name)

	if r.Spec.AgeThreshold == 0 {
		r.Spec.AgeThreshold = 24
	}

	if r.Spec.QueryTime == "" {
		r.Spec.QueryTime = "0 16 * * 1-5"
	}

	if r.Spec.QueryNamespace == "" {
		r.Spec.QueryNamespace = "default"
	}
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-hog-immortal-hedgehogs-v1-hogeye,mutating=false,failurePolicy=fail,sideEffects=None,groups=hog.immortal.hedgehogs,resources=hogeyes,verbs=create;update,versions=v1,name=vhogeye.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &HogEye{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *HogEye) ValidateCreate() (admission.Warnings, error) {
	hogeyelog.Info("validate create", "name", r.Name)

	return nil, r.validateEye()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *HogEye) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	hogeyelog.Info("validate update", "name", r.Name)

	return nil, r.validateEye()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *HogEye) ValidateDelete() (admission.Warnings, error) {
	hogeyelog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil, nil
}

func (r *HogEye) validateEye() error {
	var allErrs field.ErrorList

	if r.Spec.SlackChannels[0] != '#' {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec"), r.Spec.SlackChannels, "Slack channel must start with '#'"))
	}

	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(
		schema.GroupKind{Group: "hog.immortal.hedgehogs", Kind: "HogEye"},
		r.Name, allErrs)
}
