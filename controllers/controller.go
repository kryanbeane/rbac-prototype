package controllers

import (
	"context"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type RbacPrototypeController struct {
	client.Client
	*runtime.Scheme
}

const (
	deploymentControllerName = "deployment-controller"
	serviceAccountName       = "rbac-prototype-service-account"
	roleName                 = "rbac-prototype-role"
	roleBindingName          = "rbac-prototype-binding"
)

var _ reconcile.Reconciler = &RbacPrototypeController{}

//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=authorization.k8s.io,resources=roles,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete

func (p RbacPrototypeController) Add(mgr manager.Manager) error {
	// Create a new Controller
	c, err := controller.New(deploymentControllerName, mgr,
		controller.Options{Reconciler: &RbacPrototypeController{
			Client: mgr.GetClient(),
			Scheme: mgr.GetScheme(),
		}})
	if err != nil {
		logrus.Errorf("failed to create deployment controller: %v", err)
		return err
	}

	labelSelectorPredicate, err := predicate.LabelSelectorPredicate(
		v1.LabelSelector{
			MatchLabels: map[string]string{
				"rbac": "prototype",
			},
		},
	)
	if err != nil {
		logrus.Errorf("error creating label selector predicate: %v", err)
		return err
	}

	// Add a watch to Deployments containing that label and enqueue the Deployment object key
	err = c.Watch(
		&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForObject{}, labelSelectorPredicate)
	if err != nil {
		logrus.Errorf("error creating watch for deployments: %v", err)
		return err
	}

	return nil
}

//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=authorization.k8s.io,resources=roles,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=leases,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;patch;delete

func (p RbacPrototypeController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {

	serviceAccount := corev1.ServiceAccount{
		ObjectMeta: v1.ObjectMeta{
			Name:      serviceAccountName,
			Namespace: request.Namespace,
		},
	}

	role := rbacv1.ClusterRole{
		ObjectMeta: v1.ObjectMeta{
			Name: roleName,
		},
	}

	roleBinding := rbacv1.ClusterRoleBinding{
		ObjectMeta: v1.ObjectMeta{
			Name: roleBindingName,
		},
	}

	var deployment appsv1.Deployment

	// Get the found deployment
	if err := p.Client.Get(ctx, request.NamespacedName, &deployment); err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}
	logrus.Infof("reconciling deployment %s", deployment.Name)

	// Check if the service account is already assigned
	if deployment.Spec.Template.Spec.ServiceAccountName != serviceAccountName {
		// Check if the service account already exists
		err := p.Client.Get(ctx, client.ObjectKeyFromObject(&serviceAccount), &serviceAccount)
		if err != nil {
			// If it doesn't exist, create it
			if errors.IsNotFound(err) {
				if err := p.Client.Create(ctx, &corev1.ServiceAccount{
					ObjectMeta: v1.ObjectMeta{
						Name:      serviceAccountName,
						Namespace: request.Namespace,
					},
				}); err != nil {
					logrus.Errorf("failed to create service account: %v", err)
					return reconcile.Result{}, err
				}
				logrus.Infof("created service account %s", serviceAccountName)
			}
			// Other error thrown while getting service account
			logrus.Errorf("error getting service account %s: %v", serviceAccountName, err)
			return reconcile.Result{}, err
		}
	}
	logrus.Infof("created new service account named %s", serviceAccountName)

	// Assign the service account to the requesting Deployment
	deployment.Spec.Template.Spec.ServiceAccountName = serviceAccountName
	if err := p.Client.Update(ctx, &deployment); err != nil {
		logrus.Errorf("failed to update deployment with new service account: %v", err)
		return reconcile.Result{}, err
	}
	logrus.Infof("updated deployment %s with new service account %s", deployment.Name, serviceAccountName)

	// Check if the role already exists
	if err := p.Client.Get(ctx, client.ObjectKeyFromObject(&role), &role); err != nil {
		// If it doesn't exist, create it
		if errors.IsNotFound(err) {
			if err := p.Client.Create(ctx, &rbacv1.ClusterRole{
				ObjectMeta: v1.ObjectMeta{
					Name: roleName,
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups: []string{"coordination.k8s.io"},
						Resources: []string{"leases"},
						Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
					},
				},
			}); err != nil {
				logrus.Errorf("failed to create role: %v", err)
				return reconcile.Result{}, err
			}
			logrus.Infof("created role %s", roleName)
		}
		logrus.Errorf("error getting role %s: %v", roleName, err)
		return reconcile.Result{}, err
	}
	logrus.Infof("created new role named %s", roleName)

	// Check if the role binding already exists
	if err := p.Client.Get(ctx, client.ObjectKeyFromObject(&roleBinding), &roleBinding); err != nil {
		if errors.IsNotFound(err) {
			// Create role binding
			if err := p.Client.Create(ctx, &rbacv1.ClusterRoleBinding{
				ObjectMeta: v1.ObjectMeta{
					Name: roleBindingName,
				},
				Subjects: []rbacv1.Subject{
					{
						Kind:      rbacv1.ServiceAccountKind,
						Name:      serviceAccountName,
						Namespace: request.Namespace,
					},
				},
				RoleRef: rbacv1.RoleRef{
					Kind:     "ClusterRole",
					Name:     roleName,
					APIGroup: "rbac.authorization.k8s.io",
				},
			}); err != nil {
				logrus.Errorf("failed to create role binding: %v", err)
				return reconcile.Result{}, err
			}
		} else {
			logrus.Errorf("failed to get role binding: %v", err)
			return reconcile.Result{}, err
		}
	}

	logrus.Infof("created new role binding named %s", roleBindingName)
	logrus.Info("SUCCESS: RBAC Prototype Reconcile Complete")

	return reconcile.Result{}, nil
}
