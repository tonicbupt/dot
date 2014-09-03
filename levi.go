package main

import (
	"code.google.com/p/go-uuid/uuid"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Levi struct {
	conn    *Connection
	inTask  chan *Task
	closed  chan bool
	host    string
	size    int
	tasks   map[string][]Task
	waiting map[string][]Task
}

func NewLevi(conn *Connection, size int) *Levi {
	return &Levi{conn, conn.host, size, false, make(map[string][]Task), make(map[string][]Task)}
}

func (self *Levi) WaitTask() {
	for {
		select {
		case task := <-inTask:
			key := fmt.Sprintf("%s:%s:%s", task.Name, task.Uid, task.Type)
			if _, exists := self.tasks[key]; exists {
				self.tasks[key] = append(self.tasks[key], *task)
			} else {
				self.tasks[key] = []Task{}
			}
			if self.Len() >= self.size {
				logger.Debug("full check")
				self.SendTasks()
			}
		case <-self.closed:
			break
		case <-time.After(time.Second * time.Duration(config.Task.Dispatch)):
			if self.Len() != 0 {
				logger.Debug("full check")
				self.SendTasks()
			}
		}
	}
}

func (self *Levi) SendTasks() {
	logger.Debug(self.tasks)
	for key, tasks := range self.tasks {
		go func(key string, tasks []Task) {
			keys := strings.Split(key, ":")
			// 缺失
			if len(keys) != 4 {
				return
			}
			// build tasks to send
			name, uidStr, typeStr := keys[0], keys[1], keys[2]
			id := uuid.New()
			uidInt, _ := strconv.Atoi(uidStr)
			typeInt, _ := strconv.Atoi(typeStr)
			groupedTask := GroupedTask{
				Name:  name,
				Uid:   uidInt,
				Type:  typeInt,
				Id:    id,
				Tasks: tasks,
			}
			// cache tasks
			self.waiting[id] = tasks
			// send tasks
			if err := self.conn.ws.WriteJSON(&groupedTask); err != nil {
				logger.Assert(err, "JSON write error")
			}
		}(key, tasks)
	}
}

func (self *Levi) Close() {
	self.conn.CloseConnection()
	self.closed <- true
	self.tasks = make(map[string][]Task)
}

func (self *Levi) Run() {
	// 定时检查
	go func() {
		for !self.closed {
			if self.Len() > 0 {
				logger.Debug("period check")
				self.SendTasks()
				logger.Debug("period check done")
			} else {
				logger.Debug("empty queue")
			}
			time.Sleep(time.Duration(config.Task.Dispatch) * time.Second)
		}
	}()
	// 接收数据
	go func() {
		for !self.conn.closed {
			var taskReply TaskReply
			if err := self.conn.ws.ReadJSON(&taskReply); err == nil {
				// do
				// 保存 container 之类的
				for taskUuid, taskReplies := range taskReply {
					if tasks, exists := self.waiting[taskUuid]; exists {
						if len(tasks) != len(taskReplies) {
							logger.Debug("长度不对啊这")
							continue
						}
						for i := 0; i < len(tasks); i = i + 1 {
							task := tasks[i]
							retval := taskReplies[i]
							switch task.Type {
							case AddContainer:
								app := GetApplicationByNameAndVersion(task.Name, task.Version)
								host := GetHostByIp(task.Host)
								if app == nil || host == nil {
									logger.Info("app/host 没了")
									continue
								}
								NewContainer(app, host, task.Bind, retval.(string), task.Daemon)
							case RemoveContainer:
								old := GetContainerByCid(task.Container)
								if old == nil {
									logger.Info("要删的容器已经不在了")
									continue
								}
								old.Delete()
							case UpdateContainer:
								old := GetContainerByCid(task.Container)
								if old != nil {
									old.Delete()
								}
								app := GetApplicationByNameAndVersion(task.Name, task.Version)
								host := GetHostByIp(task.Host)
								if app == nil || host == nil {
									logger.Info("app/host 没了")
									continue
								}
								NewContainer(app, host, task.Bind, retval.(string), task.Daemon)
							}
						}
					}
				}
			} else {
				logger.Debug("出错了, 关闭连接退出goroutine")
				self.conn.CloseConnection()
			}
		}
		defer func() {
			self.Close()
			hub.RemoveLevi(self.host)
		}()
	}()
}

func (self *Levi) Len() int {
	count := 0
	for _, value := range self.tasks {
		count = count + len(value)
	}
	return count
}
