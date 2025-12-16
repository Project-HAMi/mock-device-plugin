# Mock device plugin for HAMi

## Introduction

This is a Kubernetes device plugin implementation that enables the registration of virtual-devices which would normally be ignored by scheduler (i.e gpu-memory, gpu-cores, etc..)on each node. After deployment, these resources will be available on `node.status.allocatable` and `node.status.capacity`

## Prerequisites

- Kubernetes version >= v1.18

## Deployment

If the `hami-scheduler-device` ConfigMap is not deployed, it needs to be deployed first. refer [device-configmap.yaml](https://github.com/Project-HAMi/HAMi/blob/master/charts/hami/templates/scheduler/device-configmap.yaml)

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

## ManagedResources

| Devices      | Mocking Resources |
| :---        |    :----:   |
| Nvidia GPU      | nvidia.com/gpumem, nvidia.com/gpumem-percentage, nvidia.com/gpucores        |
| Hygon DCU  | hygon.com/dcumem       |
| Ascend     | huawei.com/Ascend{chip-name}-memory |

**Note:**  If the counted memory is too large, for example exceeding 120GB, it will display as 0. In this case, you can set the `memoryFactor` in `hami-scheduler-device` ConfigMap. The default value of `memoryFactor` is 1.

## Maintainer

limengxuan@4paradigm.com