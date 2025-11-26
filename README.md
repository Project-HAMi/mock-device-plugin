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

## Arguments:

```
HAMi mock device plugin for Kubernetes
./k8s-device-plugin version 
Usage:
  -alsologtostderr
    	log to standard error as well as files
  -debug
    	debug mode
  -iluvatar-core-name string
    	virtual cores for iluvatar to be allocated (default "iluvatar.ai/MR-V100.vCore")
  -iluvatar-resource-name string
    	virtual devices for iluvatar to be allocated (default "iluvatar.ai/vgpu")
  -log_backtrace_at value
    	when logging hits line file:N, emit a stack trace
  -log_dir string
    	If non-empty, write log files in this directory
  -logtostderr
    	log to standard error instead of files
  -mlu-resource-memory-name string
    	virtual memory for npu to be allocated (default "huawei.com/Ascend910-memory")
  -mlu-resource-name string
    	virtual devices for mlu to be allocated (default "cambricon.com/vmlu")
  -resource-cores string
    	cores percentage to use (default "nvidia.com/gpucores")
  -resource-mem string
    	gpu memory to allocate (default "nvidia.com/gpumem")
  -resource-mem-percentage string
    	gpu memory fraction to allocate (default "nvidia.com/gpumem-percentage")
  -stderrthreshold value
    	logs at or above this threshold go to stderr
  -v value
    	log level for V logs
  -vmodule value
    	comma-separated list of pattern=N settings for file-filtered logging
```

## ManagedResources

| Devices      | Mocking Resources |
| :---        |    :----:   |
| Nvidia GPU      | nvidia.com/gpumem, nvidia.com/gpumem-percentage, nvidia.com/gpucores        |
| Cambricon MLU   | cambricon.com/vmlu        |
| Iluvatar GPU    | iluvatar.ai/vgpu, iluvatar.ai/vCore |
| Ascend 910B     | huawei.com/Ascend910-memory |

## Maintainer

limengxuan@4paradigm.com