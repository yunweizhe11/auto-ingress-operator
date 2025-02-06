/*
Copyright 2025.

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

package controllers

import (
	"context"
	"fmt"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	networkv1alpha1 "github.com/yunweizhe11/auto-ingress-operator/api/v1alpha1"
	libpkg "github.com/yunweizhe11/auto-ingress-operator/controllers/lib"
)

// AutoIngressReconciler reconciles a AutoIngress object
type AutoIngressReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	networkv1 []*networkv1alpha1.AutoIngress
	mu        sync.Mutex
}

//+kubebuilder:rbac:groups=network.operator.com,resources=autoingresses,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=network.operator.com,resources=autoingresses/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=network.operator.com,resources=autoingresses/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the AutoIngress object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
// 当文件发生变化的时候触发
func (r *AutoIngressReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)
	//读取crd资源文件执行动作
	networkv1 := &networkv1alpha1.AutoIngress{}
	err := r.Client.Get(ctx, req.NamespacedName, networkv1)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			libpkg.Logger("error", fmt.Sprintf("获取AutoIngress规则失败:%v", err))
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
		libpkg.Logger("info", fmt.Sprintf("AutoIngress不存在:%v", err))
		return ctrl.Result{}, err
	}
	if len(networkv1.Spec.ServicePrefixes) == 0 {
		libpkg.Logger("error", "ServicePrefixes 为空")
		return ctrl.Result{}, nil
	}
	// GetAllService(ctx, r, networkv1) //针对所有的服务进行匹配进行创建Ingress
	r.networkv1 = append(r.networkv1, networkv1) //数组 支持多个规则配置
	r.ReconcileServices(ctx, networkv1)          //当规则发生变化 将变化的规则重新匹配所有service
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AutoIngressReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkv1alpha1.AutoIngress{}).Watches(
		&source.Kind{
			Type: &corev1.Service{},
		},
		handler.Funcs{
			CreateFunc: r.onCreateService,
			DeleteFunc: r.onDeleteService,
			UpdateFunc: r.onUpdateService,
		},
	).Complete(r)
}
