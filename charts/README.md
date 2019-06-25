# Helm Charts使用说明

> **视频课程地址：**[戳我开始学习](https://www.bilibili.com/video/av49387629?from=search&seid=4418298671230182069)

> 注意，所有命令需要在当前目录执行

## 通过rook部署Ceph持久化存储

### 部署rook

    helm install --namespace rook-ceph --name rook-ceph rook-ceph

### 通过rook部署集群

> 在部署集群时，各节点上需要至少有1块硬盘，且节点数量大于3个。

    git clone https://github.com/rook/rook
    cd rook/cluster/examples/kubernetes/ceph
    kubectl apply -f cluster.yaml
    kubectl get pods -n rook-ceph -w

    kubectl apply -f storageclass.yaml
    kubectl apply -f toolbox.yaml

## 部署elasticsearch

在我的环境中，我使用ceph来存储elasticsearch的数据，ceph持久化存储的storageClass为rook-ceph-block，如果你的环境与此不同，请根据你的环境在elasticsearch/values.yaml中变更storageClassName的值。

    helm install --name elasticsearch elasticsearch --namespace logging

### 更新elasticsearch配置

    helm upgrade elasticsearch elasticsearch --namespace logging

### 查看elasticsearch部署情况

    kubectl get pvc -n logging
    kubectl get pods -n logging

## 部署kibana

需要等待elasticsearch部署完成后再部署kibana。

    helm install --name kibana kibana --namespace logging

### 更新kibana配置

    helm upgrade kibana kibana --namespace logging

### 查看kibana部署情况

    kubectl get pods -n logging

## 部署Fluentd

Fluentd用于收集k8s产生的日志到elasticsearch中。

    helm install --name fluentd fluentd-elasticsearch -f fluentd-elasticsearch/values-test.yaml --namespace logging

### 更新Fluentd配置

    helm upgrade fluentd fluentd-elasticsearch -f fluentd-elasticsearch/values-test.yaml --namespace logging

## 部署Jaeger链路追踪系统

   Jaeger部署依赖于elasticsearch，在部署前需要先确保elasticsearch服务是可以的，可以根据你自己的环境在jaeger-operator/values-test.yaml中修改elasticsearch的地址。

   helm install --name jaeger jaeger-operator -f jaeger-operator/values-test.yaml --namespace tracing

### 更新Jaeger配置

   helm upgrade jaeger jaeger-operator -f jaeger-operator/values-test.yaml --namespace tracing

