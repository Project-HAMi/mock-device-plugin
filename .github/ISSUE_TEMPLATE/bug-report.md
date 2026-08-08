---
name: Bug Report
about: Report a problem encountered while using mock device plugin
labels: bug
---

<!-- Please use this template while reporting a bug and provide as much info as possible. Not doing so may result in your bug not being addressed in a timely manner. Thanks!
-->

**What happened**:

**What you expected to happen**:

**How to reproduce it (as minimally and precisely as possible)**:

**Anything else we need to know?**:

- The selected mock vendor and relevant `hami-scheduler-device` ConfigMap section
- Sanitized `hami.io/node-*-register` annotations
- Relevant node capacity and allocatable extended resources
- Relevant, time-bounded mock-device-plugin, kubelet, and scheduler log excerpts
- The minimal deployment manifest and commands used to reproduce

Before posting, include only relevant, time-bounded excerpts and remove or mask credentials, tokens, private keys, certificates, device identifiers, node or host names, workload identifiers, and internal image names.

**Environment**:
- mock-device-plugin version or commit:
- HAMi version:
- Kubernetes distribution and version:
- Container runtime and version:
- Mock vendor and resource names:
- Deployment image and tag:
- Host or emulated environment, such as Kind on macOS:
- Kernel version, if applicable:
- Others:
