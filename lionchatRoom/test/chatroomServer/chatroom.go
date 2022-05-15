package chatroomServer

import (
	"context"
	"crypto/rand"
	"errors"
	"math/big"
	"sync"

	"github.com/gorilla/websocket"
	treemap "github.com/liyue201/gostl/ds/map"
	"github.com/liyue201/gostl/ds/set"
)

const TeacherShut int = 100
const StudentShut int = 200
const TeacherAndStudentOk int = 300

type Message struct {
	Type string            `json:"Type"`
	Id   int               `json:Id`
	Data map[string]string `json:"Data"`
}

type Student struct {
	conn *websocket.Conn
	// 是否连接上
	offered bool
}

type chatroom struct {
	students *treemap.Map
	// 教师
	teacher *websocket.Conn
	// 教师读写锁
	wt sync.RWMutex
	// 聊天用websocket
	chatClient *set.Set
	// 聊天消息chan
	chatMessages chan *Message
	// websocket close locker
	closeLocker *sync.RWMutex
}

func (self *chatroom) SetTeacher(teacher *websocket.Conn) {
	// 判断如果当前的teacher并不是空，即当前已经有teacher了
	// 那么我们需要关闭当前的teacher的链接
	// 有点类似单例设计模式了
	if self.teacher != nil {
		self.wt.Lock()
		if self.teacher != nil {
			self.teacher.Close()
			self.teacher = nil
		}
		self.wt.Lock()
	}

	self.wt.Lock()
	self.teacher = teacher
	self.wt.Unlock()

	for iter := self.students.Begin(); iter.IsValid(); iter.Next() {
		stu := iter.Key().(*Student)
		stu.offered = false
		go func() {
			if self.teacher != nil {
				self.wt.Lock()
				if self.teacher != nil {
					if err := self.teacher.WriteJSON(Message{
						Type: "newPeer",
						Id:   -1,
						Data: nil,
					}); err != nil {
						self.teacher.Close()
						self.teacher = nil
					}
				}
				self.wt.Unlock()
			}
		}()
	}
}

func (self *chatroom) AddStudent(studentConn *websocket.Conn) {
	student := Student{studentConn, false}
	self.students.Insert(&student, 1)
	// 要求老师端加上一个peer
	go func() {
		if self.teacher != nil {
			self.wt.Lock()
			if self.teacher != nil {
				if err := self.teacher.WriteJSON(Message{
					Type: "newPeer",
					Id:   -1,
					Data: nil,
				}); err != nil {
					self.teacher.Close()
					self.teacher = nil
				}
			}
			self.wt.Unlock()
		}
	}()
	// 生成一个协程让学生端加上一个peer
	go func() {
		if studentConn != nil {
			studentConn.WriteJSON(Message{
				Type: "newPeer",
				Id:   -1,
				Data: nil,
			})
		}
	}()
}

// 规定，只有remove函数才可以对学生进行close
func (self *chatroom) RemoveStudent(student *Student) error {
	if iter := self.students.Find(student); iter != nil {
		stu := iter.Key().(*Student)
		self.students.Erase(stu)
		// 对学生端websocket关闭
		self.closeLocker.Lock()
		stu.conn.Close()
		stu.conn = nil
		self.closeLocker.Unlock()
		return nil
	} else {
		return errors.New("RemoveStudent: the student not exist")
	}
}

func (self *chatroom) AddChatClient(cli *websocket.Conn) {
	result, _ := rand.Int(rand.Reader, big.NewInt(100000007))
	if err := cli.WriteJSON(Message{
		Type: "chatId",
		Id:   int(result.Int64()),
		Data: nil,
	}); err != nil {
		return
	}
	self.chatClient.Insert(cli)
	go self.ClientGo(cli)
}

func (self *chatroom) OfferAnswerChannel() {
	// 存储消息
	message := Message{}
	// 进行协程控制
	ctx, cancel := context.WithCancel(context.TODO())
	// 死循环，在收到老师端口发送的offer的情况下不断进行老师和学生的配对
	for {
		// 如果老师的链接没有断开，且存在
		if self.teacher != nil && self.teacher.ReadJSON(&message) == nil {
			// 接收了offer，找一个至今还没有offer的学生
			for iter := self.students.Begin(); iter.IsValid(); iter.Next() {
				stu := iter.Key().(*Student)
				// 找到了这个学生
				if !stu.offered {
					// 先设置为true已经接受了offer的状态
					stu.offered = true
					// 开辟一个协程进行offer和answer的传递
					go func(ctx context.Context, message Message) {
						// 判断此时teacher是否改变，如果改变协程停止
						select {
						case <-ctx.Done():
							{
								stu.offered = false
								return
							}
						default:
						}
						// 传offer给学生
						if err := stu.conn.WriteJSON(message); err == nil {
							for {
								// 判断此时teacher是否改变，如果改变协程停止
								select {
								case <-ctx.Done():
									{
										stu.offered = false
										return
									}
								default:
								}
								// 从学生处读取answer
								if err := stu.conn.ReadJSON(&message); err == nil {
									// 传answer给老师
									self.wt.Lock()
									// 如果老师不存在或者老师的链接已经断开，则停止
									if self.teacher != nil {
										self.wt.Lock()
										if self.teacher != nil {
											if err := self.teacher.WriteJSON(message); err != nil {
												stu.offered = false
												// 至关重要
												self.teacher.Close()
												self.teacher = nil
											}
										}
										self.wt.Unlock()
									} else {
										stu.offered = false
									}
									self.wt.Unlock()
									break
								}
							}
						}
					}(ctx, message)
					break
				}
			}
		} else {
			// 取消所有的正在进行webrtc匹配的协程
			cancel()
			// 将所有的student都设置为没有offer的状态
			for iter := self.students.Begin(); iter.IsValid(); iter.Next() {
				stu := iter.Key().(*Student)
				stu.offered = false
			}
			// 重新生成上下文
			ctx, cancel = context.WithCancel(context.TODO())
			// 至关重要！
			// 在第一次读取一个已经断开链接的websocket.Conn时，不会报错
			// 但是第二次读取时会抛出panic
			self.teacher = nil
		}
	}
}

func (self *chatroom) ClientGo(cli *websocket.Conn) {
	defer cli.Close()
	for {
		msg := Message{}
		if err := cli.ReadJSON(&msg); err != nil {
			self.chatClient.Erase(cli)
			return
		} else {
			self.chatMessages <- &msg
		}
	}
}

func (self *chatroom) ChatChannel() {
	for {
		select {
		case msg := <-self.chatMessages:
			for iter := self.chatClient.Begin(); iter.IsValid(); iter.Next() {
				cli := iter.Value().(*websocket.Conn)
				go func(msg *Message) {
					if err := cli.WriteJSON(msg); err != nil {
						self.chatClient.Erase(cli)
					}
				}(msg)
			}
		}
	}
}

func NewChatroom() *chatroom {
	room := &chatroom{
		treemap.New(treemap.WithGoroutineSafe()),
		nil,
		sync.RWMutex{},
		set.New(set.WithGoroutineSafe()),
		make(chan *Message, 1000),
		&sync.RWMutex{},
	}
	go room.OfferAnswerChannel()
	go room.ChatChannel()
	return room
}
