// job_manager.go はジョブのキャンセル状態を管理します。
package main

import "sync"

// JobManager はジョブのキャンセル状態を管理します。
type JobManager struct {
	mu   sync.Mutex
	jobs map[string]bool
}

// NewJobManager は JobManager を生成します。
func NewJobManager() *JobManager {
	return &JobManager{jobs: make(map[string]bool)}
}

// Register はジョブを登録します（キャンセル済みフラグ = false）。
func (m *JobManager) Register(jobID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.jobs[jobID] = false
}

// Cancel はジョブのキャンセル済みフラグを true にします。
// ジョブが存在しない場合は false を返します。
func (m *JobManager) Cancel(jobID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.jobs[jobID]; !ok {
		return false
	}
	m.jobs[jobID] = true
	return true
}

// IsCancelled はジョブのキャンセル済みフラグを返します。
func (m *JobManager) IsCancelled(jobID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.jobs[jobID]
}

// Unregister はジョブを削除します。
func (m *JobManager) Unregister(jobID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.jobs, jobID)
}
