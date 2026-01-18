# 🚀 Kubernetes Backup & Restore Operator

**Production-grade, Scheduled, Retention-aware Backups for Kubernetes PVCs**

> A clean, safe, and observable Kubernetes Operator for **automated backups**, **retention cleanup**, and **restores**, built with **controller-runtime best practices**.

---

## ✨ Why This Exists

Backing up Kubernetes workloads is **not just about running a cron job**.

Production systems require:

- **Safety** – protect live data at all times
- **Reliability** – deterministic behavior and retries
- **Observability** – events, metrics, and status
- **Automation** – scheduling and retention
- **Great UX** – `kubectl get` and `kubectl describe` must tell the full story

This operator is designed with **real-world production operators** in mind.

---

## 🧠 Core Concepts

| Resource         | Responsibility                              |
| ---------------- | ------------------------------------------- |
| **BackupPolicy** | Defines backup schedule and retention rules |
| **Backup**       | Represents a single backup execution        |
| **Restore**      | Restores data from an existing backup       |

All are implemented as **first-class Kubernetes APIs**.

---

## ✅ Feature Summary

### 🔁 Scheduled Backups

- Cron-based schedules (`"* * * * *"`, `"0 2 * * *"`, etc.)
- Cron parsing with accurate next-run calculation
- Tracks:
  - `lastBackupTime`
  - `nextScheduledBackup`
- Uses **`RequeueAfter`** for efficient scheduling (no polling)

---

### 🧹 Retention Cleanup

- Automatic cleanup based on `keepLast`
- Deletes only **completed backups**
- Never deletes running or failed backups
- Cleanup triggered immediately on backup completion

```yaml
retention:
  keepLast: 3
```

### 📦 Backup Execution

- Each Backup creates a Kubernetes Job
- Source PVC mounted **read-only**
- Backup written as `tar.gz` to shared storage
- Clear lifecycle:

  - `Pending → Running → Completed / Failed`

---

### ♻️ Restore Support

- Restore from completed backups only
- Validation before restore execution
- Restore jobs tracked with status and conditions

---

## 🛡️ Safety Guarantees

This operator follows defensive engineering principles:

- Read-only source PVC mounts
- OwnerReferences for garbage collection
- Idempotent reconciliation
- Explicit phase transitions
- Validation before destructive actions

---

## 🔍 Observability

### 📣 Kubernetes Events

Human-readable events available via:

```bash
kubectl describe backuppolicy <name>
kubectl describe backup <name>

Events:
  Normal  BackupStarted     Backup execution started
  Normal  JobCreated        Created backup job my-backup-job
  Normal  BackupCompleted   Backup completed successfully in 6s
  Normal  CleanupTriggered  Deleted 2 old backups (keepLast=3)
```

## 🧩 Architecture Overview

```
BackupPolicy
   │
   ├── creates ──▶ Backup
   │                  │
   │                  ├── creates ──▶ Job
   │                  │                  └── writes tar.gz
   │                  │
   │                  └── updates status, events, metrics
   │
   └── retention cleanup on backup completion
```

## 🔐 RBAC & Security

- Least-privilege RBAC
- Explicit permissions for:
  - Backups and statuses
  - Jobs
  - PVC reads
  - Backup deletions (retention)

Generated via kubebuilder annotations.

---

## 🧪 Production Patterns Used

- controller-runtime reconciliation loop
- Child resource ownership and watches
- RequeueAfter-based scheduling
- Race-condition safe Job creation
- Clear terminal states

---

## ⚠️ Known Limitations (Planned)

- Backup integrity verification
- Restore safety checks (non-empty PVC protection)
- Prometheus alerts
- Grafana dashboards

---

## 🎯 Design Philosophy

This project prioritizes:

- Clarity over cleverness
- Safety over shortcuts
- Observability over assumptions
- Kubernetes-native design

---

## 🏁 Summary

This operator provides:

- Automated scheduled backups
- Retention-aware cleanup
- Safe restore workflows
- First-class observability (events + metrics)
- Clean, production-grade controller architecture

---

_Built with care, clarity, and production realities in mind._
