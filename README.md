# k8s auto ingress operator

为 srv 和 web 开头的 service 创建对应的 ingress

域名规则: `<serviceName>---<namespace>.<autoIngressName>`


```bash
[root@k8s-master opt]# kubectl get svc
NAMESPACE       NAME                                 TYPE        CLUSTER-IP      EXTERNAL-IP   PORT(S)                      AGE
default         kubernetes                           ClusterIP   10.96.0.1       <none>        443/TCP                      50d
default         srv-my-service1                      ClusterIP   10.99.16.237    <none>        89/TCP                       14d


[root@k8s-master opt]# kubectl get ingress
NAMESPACE   NAME                        CLASS   HOSTS                                ADDRESS         PORTS   AGE
default     ingress-http                nginx   nginx.test.com                       10.106.239.94   80      16d
default     srv-my-service1--sodev-cc   nginx   srv-my-service1---default.sodev-cc   10.106.239.94   80      14d

```
## Kubernetes版本

k8s >= 1.30

## 发布配置

1. 安装控制器

```
kubectl apply -f release/k8s-auto-ingress-operator.yml
```

2. 创建域名规则

```bash
kubectl apply -f deploy/demo.yaml
```

annotations 中定义的标签可以认为是公共标签， 最终将被继承到生成的 Ingress 中。 因此可以通过 annotation 选择的 IngressController， 并为该 Controller 配置一些公共标签。


配置文件如下

```yaml
# demo.yaml
apiVersion: network.operator.com/v1palpha1
kind: AutoIngress
metadata:
  name: sodev-cc
  namespace: ingress-nginx
spec:
  ingressClassName: nginx
  rootDomain: sodev-cc
  servicePrefixes:
    - "web-"
    - "srv-"
  tlsSecretName: ""

```

+ `rootDomain`: （必须） 后缀域名, 必须。
+ `servicePrefixes`: 指定适配以 **特定** 关键字开头的 service。 默认值为 `web- / srv-`。
+ `tlsSecretName`: （可选） 指定使用的 https 证书在 k8s 集群中的名字。

<!--
## 遗留问题

控制器启动时会获取所有的 service 。 如果这个时候没有 **域名规则** ， 将不会创建 ingress 规则。
-->