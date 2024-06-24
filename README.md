# Mock device plugin for HAMi

## Introduction
This is a [Kubernetes][k8s] [device plugin][dp] implementation that enables the registration of virtual-devices which would normally be ignored by scheduler (i.e gpu-memory, gpu-cores, etc..)on each node. After deployment, these resources will be available on node.status.allocatable and node.status.capacity



## Limitations
* This plugin targets Kubernetes v1.18+.

## Deployment
```
$ kubectl apply -f k8s-mock-rbac.yaml
$ kubectl apply -f k8s-mock-plugin.yaml
```

## Build
```
docker build .
```

## Examples

```
Allocatable:
  ...
  memory:                             769189866507
  nvidia.com/gpu:                     20
  nvidia.com/gpucores:                200
  nvidia.com/gpumem:                  65536
  nvidia.com/gpumem-percentage:       200
  pods:                               110
  ...
```

## Maintainer

limengxuan@4paradigm.com