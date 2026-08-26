# Mock device plugin for HAMi
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2FProject-HAMi%2Fmock-device-plugin.svg?type=shield)](https://app.fossa.com/projects/git%2Bgithub.com%2FProject-HAMi%2Fmock-device-plugin?ref=badge_shield)

## Introduction

This is a Kubernetes device plugin that registers **virtual** device resources (e.g. `gpu-memory`, `gpu-cores`) -- the resources HAMi tracks but the kubelet would normally ignore -- onto a node, **without requiring real hardware**. It lets you test the HAMi scheduler (scheduling policies, quotas, webhooks) on machines that have no GPU/NPU.

After deployment these resources show up under `node.status.allocatable` and `node.status.capacity`.

## How it works (read this first)

The mock plugin **does not detect hardware**. It does the following every ~30s:

```text
  node annotation: hami.io/node-<vendor>-register = [ {devmem, devcore, ...}, ... ]   (1)
  node capacity:   <count resource> (e.g. nvidia.com/gpu) > 0                          (2)  <- health gate
                              | mock reads
                              v
  registers into allocatable: <vendor>/...mem , <vendor>/...cores , ...
```

On a **real** cluster, (1) and (2) are produced by the real device plugin. In a **mock-only** (no hardware) environment you provide them yourself:

- **(1) the `node-<vendor>-register` annotation** describing the fake cards -- `kubectl annotate`.
- **(2) the count extended resource** (e.g. `nvidia.com/gpu`) -- patched onto the node `status`.

> There is **no auto-labeller** in this repo, so (1) and (2) are manual today. Forgetting them is the usual cause of `device xxx is unhealthy` / `no allocation` -- see issues #14 / #16.

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
- The **count extended resource is only a health gate**: its value just needs to be `> 0`. It does **not** affect the registered memory/cores. For a production-shaped scheduling test, set it to `cards x splits-per-card` (for example, one AMD card with the default split count advertises `10`), even though `1` is enough for the mock memory/core reducer to appear.

Annotation entry fields:

| field | meaning |
| :-- | :-- |
| `id` | unique device UUID (any string) |
| `devmem` | per-card memory in MiB -- **summed** into `...-memory` |
| `devcore` | per-card cores. **NVIDIA/Hygon:** summed into `...-cores` (NVIDIA: percentage, 100 = a whole card). **AMD:** physical CU count used by HAMi when converting percentage requests, but the mock registers **100 per healthy card**. **Ascend:** ignored -- `huawei.com/<chip>-core` is percentage-based, registered as **100 per card**. |
| `count` | per-card split/concurrency limit used by HAMi Scheduler; the mock memory/core reducer does not sum it |
| `type` | device model string |
| `health` | must be `true` to be counted |
| `index` | card index `0,1,2,...` (`0` may be omitted) |
| `numa`, `mode` | optional |

> **Worked example (Ascend, below):** the annotation has **2 entries**, each `devmem=32768`. So `...-memory = 2x32768 = 65536` and `...-core = 2x100 = 200` (Ascend `...-core` is **percentage-based**: a whole card is always **100**). The count resource `=8` is a separate health-gate value and is unrelated to these numbers.

## Usage by vendor

To mock one card you always need the **three pieces**: the vendor **config block** (in the `hami-scheduler-device` ConfigMap, gives the resource names), the **count extended resource** (passes the health gate), and the **node-register annotation** (describes the fake cards). Replace `<node>` below with your target node. Resource names follow the ConfigMap (HAMi defaults shown). See [Understanding the values](#understanding-the-values) for how the numbers are derived.

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

### AMD GPU

- config block: `amd:` | annotation: `hami.io/node-amd-register` (JSON) | count: `amd.com/gpu`
- mock registers: `amd.com/gpumem`, `amd.com/gpucores`

The AMD config must declare all three distinct resource names under the same vendor namespace:

```yaml
amd:
  resourceCountName: amd.com/gpu
  resourceMemoryName: amd.com/gpumem
  resourceCoreName: amd.com/gpucores
```

```bash
# (2) external count resource: one card x the AMD plugin's default split count (10)
kubectl patch node <node> --subresource=status --type=json \
  -p '[{"op":"add","path":"/status/capacity/amd.com~1gpu","value":"10"}]'
# (1) transport-safe fake inventory for a Scheduler smoke test. devmem is
# intentionally reduced to 32 GiB; see the MI300X limitation below.
kubectl annotate node <node> \
  'hami.io/node-amd-register=[{"id":"AMD-MOCK-0","count":10,"devmem":32768,"devcore":304,"type":"AMD-MOCK-32G","health":true,"numa":0}]'
# verify (~30s later)
kubectl get node <node> -o json | jq '.status.allocatable|with_entries(select(.key|test("amd.com")))'
# expect: amd.com/gpu=10, amd.com/gpumem=32768, amd.com/gpucores=100
```

HAMi expresses AMD core requests as percentages. Each healthy physical card therefore contributes `100` to `amd.com/gpucores`; the annotation's `devcore` remains the real CU count that HAMi uses to convert the requested percentage into CUs. Unhealthy annotation entries contribute neither memory nor core capacity.

The released AMD device plugin reports one MI300X VF with `count: 10`, about `196288` MiB of memory, and `devcore: 304`. The current generic mock expands every MiB of scalar capacity into one Device Plugin API object; a full MI300X response is therefore about 6.76 MiB and exceeds kubelet's default 4 MiB gRPC receive limit.

> [!IMPORTANT]
> This is a mock-only transport limitation. The released AMD device plugin registers only ten logical `amd.com/gpu` splits per physical card and carries the full memory/CU values in the Node annotation; it does not send 196288 memory Device objects to kubelet. Do not use this PR to claim full-capacity MI300X mock coverage.

HAMi's AMD configuration does not yet expose an end-to-end `memoryFactor`, so do not use a full MI300X memory value as a working mock example. Supporting that capacity without changing resource semantics requires a matching factor in both HAMi Scheduler and this mock. The reduced 32 GiB inventory above verifies only annotation decoding, resource registration, and Scheduler Filter/Bind behavior.

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
| AMD GPU    | `amd.com/gpumem`, `amd.com/gpucores` |
| Hygon DCU  | `hygon.com/dcumem`, `hygon.com/dcucores` (when `resourceCoreName` is set) |
| Ascend     | `huawei.com/Ascend{chip}-memory`, `huawei.com/Ascend{chip}-core` (when `resourceCoreName` is set **and** the node is in `hami-vnpu-core` mode) |

**Note:** If counted memory is too large (e.g. > 120GB), the Device Plugin response may exceed kubelet's receive limit and the resource can display as `0`. Device families whose HAMi configuration exposes `memoryFactor` can reduce the advertised scalar count with the same factor. AMD does not currently have that end-to-end setting; use the reduced smoke-test inventory above until the Scheduler and mock contracts are extended together.

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
