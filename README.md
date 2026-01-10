# Kubernetes Backup & DR Operator

A Kubernetes operator for backing up and restoring stateful workloads. Built as a learning project to understand operator patterns, custom resources, and safe state management in Kubernetes.

## Problem Statement

Stateful applications (databases, persistent volumes, config) need protection. While cloud providers offer snapshot APIs and enterprise tools exist, there's value in understanding **how** backup orchestration works at the Kubernetes level.

This operator automates:

- Scheduled backups of PersistentVolumeClaims (PVCs)
- On-demand backup triggers
- Restore operations with safety checks
- Backup lifecycle management (retention, cleanup)

## Goals

- **Learn by building**: Understand CRDs, controllers, reconciliation loops, and the operator pattern
- **Safe operations**: Validate state before mutating workloads; fail safe, not silent
- **Observable**: Clear status conditions, events, and logs
- **Realistic scope**: Focus on core backup/restore patterns, not enterprise features

## Non-Goals

- **Multi-cluster DR**: Single-cluster only
- **Cloud-native snapshots**: Use generic copy mechanisms (rsync, tar) rather than CSI snapshots
- **Production-grade performance**: Prioritize clarity over optimization
- **High availability**: Single controller replica is fine
- **Advanced scheduling**: Simple cron-like scheduling, no complex dependency graphs

## High-Level Architecture

```
┌─────────────────────────────────────────────────────────┐
│                   User / Platform Team                   │
└───────────────────┬─────────────────────────────────────┘
                    │ kubectl apply -f backup.yaml
                    ▼
┌─────────────────────────────────────────────────────────┐
│              Custom Resources (CRDs)                     │
│  • BackupPolicy  - When and what to backup               │
│  • Backup        - A single backup instance              │
│  • RestoreJob    - Restore operation                     │
└───────────────────┬─────────────────────────────────────┘
                    │ Watches for changes
                    ▼
┌─────────────────────────────────────────────────────────┐
│           Backup Operator (Controller)                   │
│  • Reconciles BackupPolicy → creates Backups             │
│  • Reconciles Backup → executes backup job               │
│  • Reconciles RestoreJob → executes restore              │
│  • Updates .status with results                          │
└───────────────────┬─────────────────────────────────────┘
                    │ Creates/manages
                    ▼
┌─────────────────────────────────────────────────────────┐
│              Kubernetes Resources                        │
│  • Jobs (for backup/restore execution)                   │
│  • ConfigMaps (for backup metadata)                      │
│  • PVCs (backup storage targets)                         │
└─────────────────────────────────────────────────────────┘
```

### Key Concepts (You'll Learn These)

- **CRD (Custom Resource Definition)**: Teaches Kubernetes about new resource types (like `Backup`)
- **Controller**: Watches CRDs and makes the cluster match desired state
- **Reconciliation Loop**: "Something changed → read current state → make it match desired state"

## Current Status

🚧 **Project Setup Phase**

- [ ] Repository structure
- [ ] README and goals defined
- [ ] Go module initialized
- [ ] Controller scaffolding
- [ ] First CRD defined
- [ ] First controller implemented
- [ ] Basic testing

## How to Use (Future)

```yaml
# Define a backup policy
apiVersion: backup.example.com/v1alpha1
kind: BackupPolicy
metadata:
  name: database-backups
spec:
  schedule: "0 2 * * *" # Daily at 2 AM
  target:
    pvcName: postgres-data
    namespace: production
  retention:
    keepLast: 7
```

The operator will automatically create `Backup` resources on schedule.

## Prerequisites

- Go 1.21+
- Kubernetes cluster (kind, minikube, or real cluster)
- kubectl configured

## Development

```bash
# Initialize Go module
go mod init github.com/yourusername/backup-operator

# Run locally against your kubeconfig cluster
make run

# Build and deploy to cluster
make deploy
```

## Learning Resources

This project is built following:

- [Kubebuilder Book](https://book.kubebuilder.io/)
- [Operator Pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)
- [Sample Controller](https://github.com/kubernetes/sample-controller)

## License

MIT (or your choice)

## Acknowledgments

Built as a learning project. Inspired by Velero, Stash, and the Kubernetes community's operator patterns.
