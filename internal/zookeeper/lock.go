// internal/zookeeper/lock.go
package zookeeper

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/go-zookeeper/zk"
)

const (
	lockRoot = "/distributed_locks" // 所有分布式锁的根节点
)

// DistributedLock 定义了一个分布式锁对象
type DistributedLock struct {
	conn     *Conn  // ZooKeeper连接
	path     string // 锁的路径，例如 /distributed_locks/item-123
	lockNode string // 成功获取锁后，自己创建的节点路径
}

// NewDistributedLock 创建一个新的分布式锁实例
func NewDistributedLock(conn *Conn, resourceID string) *DistributedLock {
	// 确保根节点存在
	// 在生产环境中，这个操作通常由初始化脚本完成
	if _, _, err := conn.Exists(lockRoot); err != nil {
		_, createErr := conn.Create(lockRoot, []byte(""), 0, zk.WorldACL(zk.PermAll))
		if createErr != nil && createErr != zk.ErrNodeExists {
			// 如果创建失败且不是因为节点已存在，则是一个严重问题
			panic(fmt.Sprintf("Failed to create lock root node: %v", createErr))
		}
	}

	lockPath := lockRoot + "/" + resourceID

	// 确保锁的父节点路径存在
	if _, _, err := conn.Exists(lockPath); err != nil {
		_, createErr := conn.Create(lockPath, []byte(""), 0, zk.WorldACL(zk.PermAll))
		if createErr != nil && createErr != zk.ErrNodeExists {
			// 如果创建失败且不是因为节点已存在，则是一个严重问题
			panic(fmt.Sprintf("Failed to create lock path node %s: %v", lockPath, createErr))
		}
	}

	return &DistributedLock{
		conn: conn,
		path: lockPath,
	}
}

// Lock 尝试获取锁，如果获取不到则阻塞等待
func (l *DistributedLock) Lock() error {
	// 1. 在锁路径下创建一个临时顺序节点
	// 格式为: /distributed_locks/resourceID/lock-
	nodePath, err := l.conn.CreateProtectedEphemeralSequential(l.path+"/lock-", []byte(""), zk.WorldACL(zk.PermAll))
	if err != nil {
		return fmt.Errorf("failed to create sequential node: %w", err)
	}
	l.lockNode = nodePath

	for {
		// 2. 获取锁路径下的所有子节点
		children, _, err := l.conn.Children(l.path)
		if err != nil {
			return fmt.Errorf("failed to get children nodes: %w", err)
		}
		sort.Strings(children) // 排序，保证顺序

		// 3. 判断自己是否是最小的节点
		myNodeName := strings.TrimPrefix(l.lockNode, l.path+"/")
		if myNodeName == children[0] {
			// 是最小节点，成功获取锁
			return nil
		}

		// 4. 不是最小节点，监听前一个节点
		prevNodeIndex := -1
		for i, child := range children {
			if child == myNodeName {
				prevNodeIndex = i - 1
				break
			}
		}
		if prevNodeIndex < 0 {
			return errors.New("cannot find previous node, something is wrong")
		}
		prevNodePath := l.path + "/" + children[prevNodeIndex]

		// 使用 ExistsW 来设置一次性的Watcher
		_, _, eventChan, err := l.conn.ExistsW(prevNodePath)
		if err != nil {
			// 如果在前一个节点检查时它刚好被删除了，就重试循环
			if err == zk.ErrNoNode {
				continue
			}
			return fmt.Errorf("failed to watch previous node: %w", err)
		}

		// 阻塞等待事件
		select {
		case event := <-eventChan:
			// 如果前一个节点被删除，我们就收到通知，重新进入循环去竞争锁
			if event.Type == zk.EventNodeDeleted {
				continue
			}
		case <-time.After(30 * time.Second): // 设置超时，防止死等
			return errors.New("timeout waiting for lock")
		}
	}
}

// Unlock 释放锁
func (l *DistributedLock) Unlock() error {
	if l.lockNode == "" {
		return errors.New("no lock to unlock")
	}
	err := l.conn.Delete(l.lockNode, -1)
	if err != nil && err != zk.ErrNoNode {
		return fmt.Errorf("failed to delete lock node: %w", err)
	}
	l.lockNode = ""
	return nil
}
