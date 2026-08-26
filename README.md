# Mock device plugin for HAMi
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2FProject-HAMi%2Fmock-device-plugin.svg?type=shield)](https://app.fossa.com/projects/git%2Bgithub.com%2FProject-HAMi%2Fmock-device-plugin?ref=badge_shield)

## Introduction

This is a Kubernetes device plugin that registers **virtual** device resources (e.g. `gpu-memory`, `gpu-cores`) -- the resources HAMi tracks but the kubelet would normally ignore -- onto a node, **without requiring real hardware**. It lets you test the HAMi scheduler (scheduling policies, quotas, webhooks) on machines that have no GPU/NPU.

After deployment these resources show up under `node.status.allocatable` and `node.status.capacity`.

## How it works (read this first)

The mock plugin **does not detect hardware**. It does the following every ~30s:

```text
  node annotation: hami.io/node-<vendor>-register = [ {devmem, devcore, ...}, ... ]   (1)
  node capacity:   <count resource> (e.g. nvidia.com/gpu) > 0                          (2)  <- health gate for supplemental resources
                              | mock reads
                              v
  registers into allocatable: <vendor>/...mem , <vendor>/...cores , ...
```

On a **real** cluster, each family's required inputs are produced by the real device plugin. In a **mock-only** (no hardware) environment you provide them yourself:

- **(1) the `node-<vendor>-register` annotation** describing the fake cards -- `kubectl annotate`.
- **(2) the count extended resource** (e.g. `nvidia.com/gpu`) -- patched onto the node `status` for NVIDIA, Hygon and Ascend.

Kunlun vXPU uses only its `hami.io/node-register-xpu` annotation. The mock derives both `kunlunxin.com/vxpu` and `kunlunxin.com/vxpu-memory` from healthy annotation entries, so it does not need a separate count-resource status patch.

> There is **no auto-labeller** in this repo, so the required inputs are manual today. Forgetting them is the usual cause of `device xxx is unhealthy` / `no allocation` -- see issues #14 / #16.

## Prerequisites

- Kubernetes >= v1.18
- The `hami-scheduler-device` ConfigMap (the device config). If HAMi is installed it already exists; otherwise create it from [device-configmap.yaml](https://github.com/Project-HAMi/HAMi/blob/master/charts/hami/templates/scheduler/device-configmap.yaml).

## Deployment

```bash
make deploy        # = kubectl apply -f k8s-mock-rbac.yaml && kubectl apply -f k8s-mock-plugin.yaml
# or manually:
kubectl apply -f k8s-mock-rbac.yaml
kubectl apply -f k8s-mock-plugin.yaml
```

## Understanding the values

The mock **derives the registered resources from the annotation, not from the count resource.** This trips people up, so to be explicit:

- **Number of fake cards = the number of entries in the annotation array** (not the count value).
- Registered `...-memory` = **sum of `devmem`** over all entries.
- Registered `...-cores` / `...-core` = **sum of `devcore`** over all entries.
- For NVIDIA, Hygon and Ascend, the **count extended resource is only a health gate**: its value just needs to be `> 0`. It does **not** affect the registered memory/cores. By convention it is set to `cards x splits-per-card` (e.g. Ascend `2 x VDeviceCount(4) = 8`), but `1` would work equally well for the memory/cores to appear.
- Kunlun vXPU is fully annotation-derived: `vxpu = sum(count)` and `vxpu-memory = sum(devmem)` across entries whose `health` is `true`.

Annotation entry fields:

| field | meaning |
| :-- | :-- |
| `id` | unique device UUID (any string) |
| `devmem` | per-card memory in MB -- **summed** into `...-memory` |
| `devcore` | per-card cores. **NVIDIA/Hygon:** summed into `...-cores` (NVIDIA: percentage, 100 = a whole card). **Ascend:** ignored -- `huawei.com/<chip>-core` is percentage-based, registered as **100 per card**. |
| `count` | per-card split count. **Kunlun:** summed into `vxpu`; informational for the other mocks. |
| `type` | device model string |
| `health` | scheduler-visible health; only healthy Kunlun entries contribute to its mock resources |
| `index` | card index `0,1,2,...` (`0` may be omitted) |
| `numa`, `mode` | optional |

> **Worked example (Ascend, below):** the annotation has **2 entries**, each `devmem=32768`. So `...-memory = 2x32768 = 65536` and `...-core = 2x100 = 200` (Ascend `...-core` is **percentage-based**: a whole card is always **100**). The count resource `=8` is a separate health-gate value and is unrelated to these numbers.

## Usage by vendor

NVIDIA, Hygon and Ascend need the vendor **config block**, the **count extended resource** health gate and the **node-register annotation**. Kunlun vXPU needs its config block and node-register annotation, but not a separate status patch. Replace `<node>` below with your target node. Resource names follow the ConfigMap (HAMi defaults shown). See [Understanding the values](#understanding-the-values) for how the numbers are derived.

### NVIDIA (e.g. A100-80GB)

- config block: `nvidia:` | annotation: `hami.io/node-nvidia-register` (JSON) | count: `nvidia.com/gpu`
- mock registers: `nvidia.com/gpumem`, `nvidia.com/gpucores`, `nvidia.com/gpumem-percentage`

```bash
# (2) count resource: splitCount=10 -> one card advertises 10
kubectl patch node <node> --subresource=status --type=json \
  -p '[{"op":"add","path":"/status/capacity/nvidia.com~1gpu","value":"10"}]'
# (1) annotation: 1 x A100-80G (devmem=81920 MB, devcore=100)
kubectl annotate node <node> \
  'hami.io/node-nvidia-register=[{"id":"GPU-MOCK-0","count":10,"devmem":81920,"devcore":100,"type":"NVIDIA-A100-SXM4-80GB","health":true,"numa":0,"mode":"hami-core"}]'
# verify (~30s later)
kubectl get node <node> -o json | jq '.status.allocatable|with_entries(select(.key|test("nvidia.com")))'
# expect: nvidia.com/gpu=10, nvidia.com/gpumem=81920, nvidia.com/gpucores=100, nvidia.com/gpumem-percentage=100
```

### Ascend NPU (e.g. 910B4)

- config block: `vnpus.configs` (entry for `910B4`) | annotation: `hami.io/node-register-Ascend910B4` (JSON) | count: `huawei.com/Ascend910B4`
- mock registers: `huawei.com/Ascend910B4-memory`, and -- **only when the node runs in `hami-vnpu-core` soft mode** (see below) -- `huawei.com/Ascend910B4-core`

```bash
# (2) count resource: 2 cards x VDeviceCount(4) = 8
kubectl patch node <node> --subresource=status --type=json \
  -p '[{"op":"add","path":"/status/capacity/huawei.com~1Ascend910B4","value":"8"}]'
# (1) annotation: 2 x 910B4 (devmem=32768 MB, devcore=20), matching the real ascend-device-plugin report
kubectl annotate node <node> \
  'hami.io/node-register-Ascend910B4=[{"id":"MOCK-0","count":4,"devmem":32768,"devcore":20,"type":"Ascend910B4","health":true},{"id":"MOCK-1","index":1,"count":4,"devmem":32768,"devcore":20,"type":"Ascend910B4","health":true}]'
# (1b) soft mode: mark the node as hami-vnpu-core so the -core resource is reported.
# Either set vnpus.hamiVnpuCore: true globally in the ConfigMap, or annotate this node
# (the node annotation overrides the global setting, same precedence as the real plugin):
kubectl annotate node <node> hami-vnpu-core=true
# verify
kubectl get node <node> -o json | jq '.status.allocatable|with_entries(select(.key|test("Ascend910B4")))'
# expect: huawei.com/Ascend910B4-memory=65536, huawei.com/Ascend910B4-core=200  (2 cards x 100)
```

The Ascend `-core` resource is **percentage-based**: each physical card contributes **100** (a whole card), independent of the annotation's `devcore`. HAMi caps a core request at 100 and, in `hami-vnpu-core` soft mode, treats a card's total core as 100.

**When is `-core` reported?** Only on a node that runs in `hami-vnpu-core` (soft-partition) mode, mirroring the real ascend-device-plugin and the HAMi scheduler. Mode is decided by: the node's `hami-vnpu-core` annotation if present, otherwise the global `vnpus.hamiVnpuCore` in the ConfigMap (`true`/`false`, default `false`). On a hard/template node (`hami-vnpu-core` off) the HAMi scheduler filters out any pod that requests `-core` (`ModeNotFit`), so the mock does not register `-core` there. There is **no ascend-device-plugin** in a mock environment, so nothing writes the `hami-vnpu-core` node annotation automatically -- set it yourself (like the `node-register` annotation and the count resource).

### Hygon DCU

- config block: `hygon:` | annotation: `hami.io/node-dcu-register` (**CSV**, not JSON) | count: `hygon.com/dcunum`
- mock registers: `hygon.com/dcumem`, and -- when the config sets `resourceCoreName` -- `hygon.com/dcucores` (sum of per-card `devcore`)

```bash
# (2) count resource: 2 DCUs
kubectl patch node <node> --subresource=status --type=json \
  -p '[{"op":"add","path":"/status/capacity/hygon.com~1dcunum","value":"2"}]'
# (1) annotation: CSV form is "id,count,devmem,devcore,type,numa,health,index,mode:" per card
kubectl annotate node <node> \
  'hami.io/node-dcu-register=DCU-MOCK-0,2,16384,100,Z100L,0,true,0,hami-core:DCU-MOCK-1,2,16384,100,Z100L,1,true,1,hami-core:'
# verify
kubectl get node <node> -o json | jq '.status.allocatable|with_entries(select(.key|test("hygon.com")))'
# expect: hygon.com/dcumem=32768, hygon.com/dcucores=200  (2 cards x devcore 100)
```

### Kunlun vXPU

- config block: `kunlun:` | annotation: `hami.io/node-register-xpu` (JSON)
- mock registers: `kunlunxin.com/vxpu`, `kunlunxin.com/vxpu-memory`
- no separate count-resource status patch is required

The default HAMi config is:

```yaml
kunlun:
  resourceCountName: kunlunxin.com/xpu
  resourceVCountName: kunlunxin.com/vxpu
  resourceVMemoryName: kunlunxin.com/vxpu-memory
```

To model one healthy P800-OAM card with four 24 GiB vXPU shares:

```bash
kubectl annotate node <node> \
  'hami.io/node-register-xpu=[{"id":"XPU-MOCK-0","count":4,"devmem":98304,"devcore":100,"type":"XPU","health":true}]'
# verify (~30s later)
kubectl get node <node> -o json | jq '.status.allocatable|with_entries(select(.key|test("kunlunxin.com")))'
# expect: kunlunxin.com/vxpu=4, kunlunxin.com/vxpu-memory=98304
```

Each healthy entry contributes its `count` value to `vxpu` and its `devmem` value in MiB to `vxpu-memory`. The mock also acknowledges HAMi Scheduler's `hami.io/node-handshake-xpu` probe, matching the real vXPU device-plugin startup protocol. Unhealthy entries remain visible to the HAMi scheduler through the annotation but do not contribute kubelet capacity, matching HAMi's `FitVXPU` health check.

## Ascend config compatibility (new vs legacy)

The Ascend `vnpus` config has two layouts and the plugin accepts **both**:

```yaml
# New (HAMi >= v2.9.0): nested object
vnpus:
  hamiVnpuCore: true          # node runs in soft mode -> the -core resource is reported
  configs:
    - chipName: 910B4
      resourceCoreName: huawei.com/Ascend910B4-core   # name of the -core resource
      ...
```
```yaml
# Legacy (older / downstream): a bare list
vnpus:
  - chipName: 910B4
    ...
```
The new nested format is tried first; if that fails it falls back to the legacy flat list; if neither matches an error is returned. This lets clusters that have not upgraded their ConfigMap keep working.

## ManagedResources

| Devices    | Mocking Resources |
| :---       | :----   |
| Nvidia GPU | `nvidia.com/gpumem`, `nvidia.com/gpumem-percentage`, `nvidia.com/gpucores` |
| Hygon DCU  | `hygon.com/dcumem`, `hygon.com/dcucores` (when `resourceCoreName` is set) |
| Ascend     | `huawei.com/Ascend{chip}-memory`, `huawei.com/Ascend{chip}-core` (when `resourceCoreName` is set **and** the node is in `hami-vnpu-core` mode) |
| Kunlun vXPU | `kunlunxin.com/vxpu`, `kunlunxin.com/vxpu-memory` |

**Note:** For device configs that expose `memoryFactor`, set it in the `hami-scheduler-device` ConfigMap if counted memory is too large (e.g. > 120GB) and displays as 0. Kunlun vXPU does not use `memoryFactor`; its `vxpu-memory` value is the direct sum of healthy entries' `devmem` values.

## Build

```bash
make build         # build the binary into bin/
make test          # run unit tests
make docker-build  # build the image (override: IMG=myrepo/mock:tag)
make help          # list all targets
```

## Maintainer

limengxuan@4paradigm.com

## License
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2FProject-HAMi%2Fmock-device-plugin.svg?type=large)](https://app.fossa.com/projects/git%2Bgithub.com%2FProject-HAMi%2Fmock-device-plugin?ref=badge_large)
