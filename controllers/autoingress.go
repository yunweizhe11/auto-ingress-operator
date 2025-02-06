package controllers

import (
	"context"
	"fmt"
	"strings"

	networkv1alpha1 "github.com/yunweizhe11/auto-ingress-operator/api/v1alpha1"
	libpkg "github.com/yunweizhe11/auto-ingress-operator/controllers/lib"
	corev1 "k8s.io/api/core/v1"
	corenetworkv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

const (
	IngressFinalizer = "network.operator.com/autoingress"
)

func (r *AutoIngressReconciler) GenIngress(svc corev1.Service, networkv1 *networkv1alpha1.AutoIngress) corenetworkv1.Ingress {
	host := fmt.Sprintf("%s---%s.%s", svc.Name, svc.Namespace, networkv1.Spec.RootDomain) //ingress host
	ingname := fmt.Sprintf("%s--%s", svc.Name, networkv1.Name)                            // ingress name
	ingspec := corenetworkv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        ingname,
			Namespace:   svc.Namespace,
			Labels:      svc.Labels,
			Annotations: networkv1.Annotations,
		},
		Spec: corenetworkv1.IngressSpec{
			IngressClassName: &networkv1.Spec.IngressClassName,
			Rules: []corenetworkv1.IngressRule{{
				Host: host,
				IngressRuleValue: corenetworkv1.IngressRuleValue{
					HTTP: &corenetworkv1.HTTPIngressRuleValue{
						Paths: []corenetworkv1.HTTPIngressPath{{
							Path:     "/",
							PathType: ptrPathType(corenetworkv1.PathTypePrefix),
							// PathType: *corenetworkv1.PathTypePrefix,
							Backend: corenetworkv1.IngressBackend{
								Service: &corenetworkv1.IngressServiceBackend{
									Name: svc.Name,
									Port: corenetworkv1.ServiceBackendPort{
										Number: svc.Spec.Ports[0].Port, //默认获取service第一个port端口
									},
								},
							},
						}},
					},
				},
			},
			},
		},
	}
	if networkv1.Spec.TlsSecretName != "" {
		ingspec.Spec.TLS = []corenetworkv1.IngressTLS{{
			Hosts:      []string{host},
			SecretName: networkv1.Spec.TlsSecretName,
		}}
	}
	return ingspec
}
func (r *AutoIngressReconciler) CreateIngress(ctx context.Context, ingspec corenetworkv1.Ingress) bool {
	Newingspec := corenetworkv1.Ingress{}
	err := r.Client.Get(ctx, client.ObjectKey{Name: ingspec.Name, Namespace: ingspec.Namespace}, &Newingspec)
	if err != nil {
		if client.IgnoreNotFound(err) == nil {
			//不存在
			err = r.Client.Create(ctx, &ingspec)
			if err != nil {
				libpkg.Logger("error", fmt.Sprintf("创建Ingress失败:%v", err))
				return false
			}
			controllerutil.AddFinalizer(&ingspec, IngressFinalizer) //add Finalizer
			if err := r.Update(ctx, &ingspec); err != nil {
				libpkg.Logger("error", fmt.Sprintf("添加Finalizer失败:%v", err))
				return false
			}
		} else {
			libpkg.Logger("error", fmt.Sprintf("Ingress信息获取失败 %s", err))
		}
	}
	libpkg.Logger("info", fmt.Sprintf("Ingress已存在:%s,触发更新动作", err))
	ingspec.SetResourceVersion(ingspec.ResourceVersion) //配置ResourceVersion
	return r.UpdateIngress(ctx, ingspec)
}

func (r *AutoIngressReconciler) DeleteIngress(ctx context.Context, IngressName string, Namespace string) error {
	ingress := &corenetworkv1.Ingress{}
	err := r.Client.Get(ctx, client.ObjectKey{Name: IngressName, Namespace: Namespace}, ingress)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			libpkg.Logger("error", fmt.Sprintf("获取Ingress失败:%v", err))
			return err
		} else {
			libpkg.Logger("info", fmt.Sprintf("Ingress不存在:%v", err))
			return nil
		}
	}
	controllerutil.RemoveFinalizer(ingress, IngressFinalizer) //remove Finalizer
	if err := r.Update(ctx, ingress); err != nil {
		libpkg.Logger("error", fmt.Sprintf("移除Finalizer失败:%v", err))
		return err
	}
	delete_err := r.Delete(ctx, ingress) //执行删除
	if delete_err != nil {
		libpkg.Logger("error", fmt.Sprintf("删除Ingress失败:%v", delete_err))
		return delete_err
	}
	libpkg.Logger("info", fmt.Sprintf("删除Ingress成功:%s", delete_err))
	return nil
}

func ptrPathType(pt corenetworkv1.PathType) *corenetworkv1.PathType {
	return &pt
}
func (r *AutoIngressReconciler) UpdateIngress(ctx context.Context, ingspec corenetworkv1.Ingress) bool {
	err := r.Client.Update(ctx, &ingspec)
	if err != nil {
		libpkg.Logger("error", fmt.Sprintf("更新Ingress失败:%v", err))
		return false
	}
	libpkg.Logger("info", fmt.Sprintf("更新Ingress成功:%v,%v", err, ingspec))
	return true
}

func (r *AutoIngressReconciler) GetService(ctx context.Context, ServiceName string, Namespace string) *corev1.Service {
	serviceSpec := &corev1.Service{}
	err := r.Client.Get(ctx, client.ObjectKey{Namespace: Namespace, Name: ServiceName}, serviceSpec)
	if err != nil {
		if client.IgnoreNotFound(err) == nil {
			libpkg.Logger("info", fmt.Sprintf("service已不存在:%v", err))
			return nil
		}
		libpkg.Logger("error", fmt.Sprintf("获取Service失败:%v", err))
		return nil
	}
	return serviceSpec
}

func checkServicePrefixes(ctx context.Context, r *AutoIngressReconciler, svc corev1.Service, networkv1 *networkv1alpha1.AutoIngress) error {
	for _, prefix := range networkv1.Spec.ServicePrefixes {
		if strings.HasPrefix(svc.Name, prefix) {
			IngressSpec := r.GenIngress(svc, networkv1)
			CreateIngressCode := r.CreateIngress(ctx, IngressSpec)
			if CreateIngressCode {
				libpkg.Logger("info", fmt.Sprintf("Ingress:%v,初始化 创建成功", IngressSpec.Name))
			} else {
				libpkg.Logger("info", fmt.Sprintf("Ingress:%v,初始化 创建失败", IngressSpec.Name))
			}
		}
		libpkg.Logger("info", fmt.Sprintf("Service:%v,不符合规则:%v", svc.Name, prefix))
	}
	return nil
}

func (r *AutoIngressReconciler) GetIngress() error {
	return nil
}

func (r *AutoIngressReconciler) onCreateService(e event.CreateEvent, q workqueue.RateLimitingInterface) {
	libpkg.Logger("info", "新 Service 被创建")
	r.mu.Lock()
	defer r.mu.Unlock()
	ctx := context.TODO()
	svc := r.GetService(context.TODO(), e.Object.GetName(), e.Object.GetNamespace())
	for _, networkv1 := range r.networkv1 {
		if networkv1 == nil {
			libpkg.Logger("error", fmt.Sprintf("规则配置 为空 %v", networkv1))
		}
		_ = checkServicePrefixes(ctx, r, *svc, networkv1) //判断符合规则就创建 不符合就skip
	}
}

func (r *AutoIngressReconciler) onUpdateService(e event.UpdateEvent, q workqueue.RateLimitingInterface) {
	libpkg.Logger("info", "更新事件")
	ctx := context.TODO()
	for _, networkv1 := range r.networkv1 {
		if e.ObjectOld.GetName() == e.ObjectNew.GetName() && e.ObjectOld.GetNamespace() == e.ObjectNew.GetNamespace() {
			//如果是同一个service 不用检查
			svc := r.GetService(context.TODO(), e.ObjectOld.GetName(), e.ObjectOld.GetNamespace())
			checkServicePrefixes(ctx, r, *svc, networkv1)
		} else {
			//如果存在修改 则删除原有ingress 重新创建新的ingress
			r.DeleteIngress(context.TODO(), fmt.Sprintf("%s--%s", e.ObjectOld.GetName(), networkv1.Name), e.ObjectOld.GetNamespace()) //删除ingress
			svc := r.GetService(context.TODO(), e.ObjectNew.GetName(), e.ObjectNew.GetNamespace())
			checkServicePrefixes(ctx, r, *svc, networkv1)
		}
	}

}
func (r *AutoIngressReconciler) onIngressDelete(e event.DeleteEvent, q workqueue.RateLimitingInterface) {
	libpkg.Logger("info", "Ingress 被删除,需要检查service是否存在 如果存在则创建出来")
	r.mu.Lock()
	defer r.mu.Lock()
	IngressName := e.Object.GetName()
	IngressNameSpec := strings.Split(IngressName, "--") //拆分出serviceName
	ServiceSpec := r.GetService(context.TODO(), IngressNameSpec[0], e.Object.GetNamespace())
	if ServiceSpec == nil {
		err := r.DeleteIngress(context.TODO(), IngressName, e.Object.GetNamespace()) //删除ingress service已不存在
		libpkg.Logger("info", fmt.Sprintf("Ingress 删除失败 %s", err.Error()))
	}

}
func (r *AutoIngressReconciler) onDeleteService(e event.DeleteEvent, q workqueue.RateLimitingInterface) {
	libpkg.Logger("info", "Service 被删除,需要同步删除对应ingress")
	// r.mu.Lock()
	// defer r.mu.Lock()
	for _, networkv1 := range r.networkv1 {
		IngressName := fmt.Sprintf("%s--%s", e.Object.GetName(), networkv1.Name)
		DeleteIngressCode := r.DeleteIngress(context.TODO(), IngressName, e.Object.GetNamespace())
		if DeleteIngressCode != nil {
			libpkg.Logger("error", fmt.Sprintf("删除Ingress失败:%v", DeleteIngressCode))
		}
	}
}

// 获取所有service 重新配置ingress 针对规则配置发生变化情况
func (r *AutoIngressReconciler) ReconcileServices(ctx context.Context, networkv1 *networkv1alpha1.AutoIngress) {
	svcs := &corev1.ServiceList{}
	err := r.Client.List(ctx, svcs)
	if err != nil {
		libpkg.Logger("error", fmt.Sprintf("获取service列表失败::%v", err))
		return
	}

	for _, svc := range svcs.Items {
		_ = checkServicePrefixes(ctx, r, svc, networkv1) //判断符合规则就创建 不符合就skip
	}
}
