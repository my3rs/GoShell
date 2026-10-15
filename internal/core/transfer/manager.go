package transfer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	sftpService "goshell/internal/core/sftp"
)

// TaskType 定义传输类型
type TaskType string

const (
	TaskTypeUpload   TaskType = "上传"
	TaskTypeDownload TaskType = "下载"
)

// TaskStatus 传输任务状态
type TaskStatus string

const (
	StatusPending   TaskStatus = "等待中"
	StatusRunning   TaskStatus = "传输中"
	StatusCompleted TaskStatus = "已完成"
	StatusFailed    TaskStatus = "失败"
	StatusCanceled  TaskStatus = "已取消"
)

// Task 传输任务实体
type Task struct {
	ID               string
	Type             TaskType
	FileName         string
	LocalPath        string
	RemotePath       string
	TotalBytes       int64
	TransferredBytes int64
	SpeedBytesSec    float64
	SpeedHuman       string
	Percent          float32
	Status           TaskStatus
	ErrorMsg         string
	CreatedAt        time.Time
	FinishedAt       time.Time

	cancelFunc context.CancelFunc
	mu         sync.RWMutex
}

// Manager 统一调度与管理后台多协程并发传输
type Manager struct {
	sftpClient *sftpService.Client
	workerNum  int
	taskQueue  chan *Task
	tasks      []*Task
	ctx        context.Context
	cancel     context.CancelFunc
	mu         sync.RWMutex
}

// NewManager 创建传输管理器，默认允许 3 个并发传输管道
func NewManager(sftp *sftpService.Client, workers int) *Manager {
	if workers <= 0 {
		workers = 3
	}

	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{
		sftpClient: sftp,
		workerNum:  workers,
		taskQueue:  make(chan *Task, 100),
		ctx:        ctx,
		cancel:     cancel,
	}

	for i := 0; i < workers; i++ {
		go m.workerLoop()
	}

	return m
}

// AddUploadTask 投递一个文件上传任务
func (m *Manager) AddUploadTask(localPath, remoteDir string) (*Task, error) {
	info, err := os.Stat(localPath)
	if err != nil {
		return nil, fmt.Errorf("读取本地文件失败: %w", err)
	}
	if info.IsDir() {
		return nil, errors.New("暂不支持直接上传文件夹")
	}

	fileName := filepath.Base(localPath)
	remotePath := filepath.ToSlash(filepath.Join(remoteDir, fileName))

	taskCtx, taskCancel := context.WithCancel(m.ctx)
	t := &Task{
		ID:         uuid.New().String(),
		Type:       TaskTypeUpload,
		FileName:   fileName,
		LocalPath:  localPath,
		RemotePath: remotePath,
		TotalBytes: info.Size(),
		Status:     StatusPending,
		CreatedAt:  time.Now(),
		cancelFunc: taskCancel,
	}

	_ = taskCtx // 保留以便扩展

	m.mu.Lock()
	m.tasks = append([]*Task{t}, m.tasks...) // 新任务放在列表前部
	m.mu.Unlock()

	select {
	case m.taskQueue <- t:
	default:
		t.Status = StatusFailed
		t.ErrorMsg = "传输队列已满"
	}

	return t, nil
}

// AddDownloadTask 投递一个文件下载任务
func (m *Manager) AddDownloadTask(remotePath, localDir string) (*Task, error) {
	info, err := m.sftpClient.Stat(remotePath)
	if err != nil {
		return nil, fmt.Errorf("获取远程文件信息失败: %w", err)
	}
	if info.IsDir() {
		return nil, errors.New("暂不支持直接下载文件夹")
	}

	fileName := filepath.Base(remotePath)
	localPath := filepath.Join(localDir, fileName)

	taskCtx, taskCancel := context.WithCancel(m.ctx)
	t := &Task{
		ID:         uuid.New().String(),
		Type:       TaskTypeDownload,
		FileName:   fileName,
		LocalPath:  localPath,
		RemotePath: remotePath,
		TotalBytes: info.Size(),
		Status:     StatusPending,
		CreatedAt:  time.Now(),
		cancelFunc: taskCancel,
	}

	_ = taskCtx

	m.mu.Lock()
	m.tasks = append([]*Task{t}, m.tasks...)
	m.mu.Unlock()

	select {
	case m.taskQueue <- t:
	default:
		t.Status = StatusFailed
		t.ErrorMsg = "传输队列已满"
	}

	return t, nil
}

// CancelTask 取消指定任务
func (m *Manager) CancelTask(id string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, t := range m.tasks {
		if t.ID == id {
			t.mu.Lock()
			if t.Status == StatusRunning || t.Status == StatusPending {
				t.Status = StatusCanceled
				if t.cancelFunc != nil {
					t.cancelFunc()
				}
			}
			t.mu.Unlock()
			return
		}
	}
}

// GetTasks 获取所有任务切片快照
func (m *Manager) GetTasks() []*Task {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Task, len(m.tasks))
	copy(result, m.tasks)
	return result
}

// GetActiveStats 获取当前活跃传输任务数量和总速率
func (m *Manager) GetActiveStats() (activeCount int, totalSpeedHuman string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var totalSpeed float64
	for _, t := range m.tasks {
		t.mu.RLock()
		if t.Status == StatusRunning {
			activeCount++
			totalSpeed += t.SpeedBytesSec
		}
		t.mu.RUnlock()
	}

	return activeCount, sftpService.FormatSize(int64(totalSpeed), false) + "/s"
}

// Close 停止所有传输任务与工作协程
func (m *Manager) Close() {
	m.cancel()
}

func (m *Manager) workerLoop() {
	for {
		select {
		case <-m.ctx.Done():
			return
		case task, ok := <-m.taskQueue:
			if !ok {
				return
			}
			m.executeTask(task)
		}
	}
}

func (m *Manager) executeTask(t *Task) {
	t.mu.Lock()
	if t.Status == StatusCanceled {
		t.mu.Unlock()
		return
	}
	t.Status = StatusRunning
	t.mu.Unlock()

	var err error
	if t.Type == TaskTypeUpload {
		err = m.doUpload(t)
	} else {
		err = m.doDownload(t)
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	t.FinishedAt = time.Now()
	t.SpeedBytesSec = 0
	t.SpeedHuman = "0 B/s"

	if err != nil {
		if t.Status != StatusCanceled {
			t.Status = StatusFailed
			t.ErrorMsg = err.Error()
		}
	} else {
		if t.Status != StatusCanceled {
			t.Status = StatusCompleted
			t.Percent = 100.0
			t.TransferredBytes = t.TotalBytes
		}
	}
}

func (m *Manager) doUpload(t *Task) error {
	src, err := os.Open(t.LocalPath)
	if err != nil {
		return fmt.Errorf("打开本地源文件失败: %w", err)
	}
	defer src.Close()

	dst, err := m.sftpClient.CreateFile(t.RemotePath)
	if err != nil {
		return fmt.Errorf("创建远程目标文件失败: %w", err)
	}
	defer dst.Close()

	return copyWithProgress(dst, src, t)
}

func (m *Manager) doDownload(t *Task) error {
	src, err := m.sftpClient.OpenFile(t.RemotePath)
	if err != nil {
		return fmt.Errorf("打开远程文件失败: %w", err)
	}
	defer src.Close()

	if err := os.MkdirAll(filepath.Dir(t.LocalPath), 0755); err != nil {
		return fmt.Errorf("创建本地目录失败: %w", err)
	}

	dst, err := os.Create(t.LocalPath)
	if err != nil {
		return fmt.Errorf("创建本地文件失败: %w", err)
	}
	defer dst.Close()

	return copyWithProgress(dst, src, t)
}

func copyWithProgress(dst io.Writer, src io.Reader, t *Task) error {
	buf := make([]byte, 64*1024) // 64KB 缓冲区
	var lastTick = time.Now()
	var lastTransferred int64

	for {
		t.mu.RLock()
		isCanceled := t.Status == StatusCanceled
		t.mu.RUnlock()
		if isCanceled {
			return errors.New("任务已被取消")
		}

		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[0:nr])
			if nw < 0 || nr < nw {
				nw = 0
				if ew == nil {
					ew = errors.New("invalid write result")
				}
			}

			atomic.AddInt64(&t.TransferredBytes, int64(nw))

			now := time.Now()
			elapsed := now.Sub(lastTick).Seconds()
			if elapsed >= 0.5 {
				currentBytes := atomic.LoadInt64(&t.TransferredBytes)
				diff := currentBytes - lastTransferred
				speed := float64(diff) / elapsed

				t.mu.Lock()
				t.SpeedBytesSec = speed
				t.SpeedHuman = sftpService.FormatSize(int64(speed), false) + "/s"
				if t.TotalBytes > 0 {
					t.Percent = float32(float64(currentBytes) / float64(t.TotalBytes) * 100)
				}
				t.mu.Unlock()

				lastTick = now
				lastTransferred = currentBytes
			}

			if ew != nil {
				return ew
			}
			if nr != nw {
				return io.ErrShortWrite
			}
		}

		if er != nil {
			if er == io.EOF {
				break
			}
			return er
		}
	}

	return nil
}
