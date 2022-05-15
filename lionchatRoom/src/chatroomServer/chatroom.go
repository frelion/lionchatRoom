package chatroomServer

import (
	"errors"
	"fmt"
	"sync"

	"github.com/gorilla/websocket"
	treemap "github.com/liyue201/gostl/ds/map"
	"github.com/liyue201/gostl/ds/set"
)

type IncrementId struct {
	locker *sync.RWMutex
	cnt    int
}

func (inc *IncrementId) CreatId() int {
	inc.locker.Lock()
	inc.cnt++
	ans := inc.cnt
	inc.locker.Unlock()
	return ans
}

// chatroom中消息的传递介质
type Message struct {
	Type string            `json:"Type"`
	Id   int               `json:Id`
	Data map[string]string `json:"Data"`
}

// 直播中的学生端
type Student struct {
	// 用于传递offer和answer的websocket链接
	conn *websocket.Conn
	// 此刻是否已经收到了offer
	offered bool
	// 对于offer操作的lock
	offerLocker *sync.RWMutex
	// 在直播间的代号
	id int
	// 是否被删除了（遍历和删除的冲突）
	deleted bool
	// 删除锁
	deletLocker *sync.RWMutex
}

// 判断是否已经收到了offer，协程安全
func (stu *Student) IsOffered() bool {
	stu.offerLocker.Lock()
	defer stu.offerLocker.Unlock()
	return stu.offered
}

// 对offer进行设定，协程安全
func (stu *Student) SetOffer(val bool) {
	stu.offerLocker.Lock()
	defer stu.offerLocker.Unlock()
	stu.offered = val
}

// 得到id代号
func (stu *Student) GetId() int {
	return stu.id
}

// 判断是否删除掉了，协程安全
func (stu *Student) IsDelete() bool {
	stu.deletLocker.Lock()
	defer stu.deletLocker.Unlock()
	return stu.deleted
}

// 对deleted设定，协程安全
func (stu *Student) SetDelete(val bool) {
	stu.deletLocker.Lock()
	defer stu.deletLocker.Unlock()
	stu.deleted = val
}

// 设置id代号
func (stu *Student) SetId(val int) {
	stu.id = val
}

// 聊天室中的客户端
type Client struct {
	// 用于发送接收消息的*websocket.Conn
	conn *websocket.Conn
	// 是否被删除了（遍历和删除的冲突）
	deleted bool
	// 删除锁
	deletLocker *sync.RWMutex
	// chatId
	chatId int
}

//判断是否删除，协程安全
func (cli *Client) IsDelete() bool {
	cli.deletLocker.Lock()
	defer cli.deletLocker.Unlock()
	return cli.deleted
}

// 对delete进行设定，协程安全
func (cli *Client) SetDelete(val bool) {
	cli.deletLocker.Lock()
	defer cli.deletLocker.Unlock()
	cli.deleted = val
}

type chatroom struct {
	// 直播间：
	// ---存储学生端的映射表，int->*Student
	students *treemap.Map
	// 用于生成学生端映射表的key
	inc *IncrementId
	// 用于删除学生端，使用学生端的互斥锁
	studentLocker *sync.RWMutex
	// ---教师端（只是一个*websocket.Conn）
	teacher *websocket.Conn
	// 用于删除教师端，使用教师端的互斥锁
	teacherLocker *sync.RWMutex

	// 聊天室
	// 存储聊天客户端的集合 *Client
	chatClient *set.Set
	// 用于存储聊天消息的chan，消息队列 *Message
	chatMessages chan *Message
	// 用于删除聊天客户端，使用聊天客户端的互斥锁
	clientLocker *sync.RWMutex
}

// 用于删除直播教师的函数，协程不安全
func (cr *chatroom) RemoveTeacher() {
	if cr.teacher != nil {
		cr.teacher.Close()
		cr.teacher = nil
		for iter := cr.students.Begin(); iter.IsValid(); iter.Next() {
			stu := iter.Value().(*Student)
			stu.offered = false
		}
	}
}

// 用于删除直播教师的函数，协程安全
func (cr *chatroom) RemoveTeacherWithGoroutineSafe() {
	if cr.teacher != nil {
		cr.teacherLocker.Lock()
		defer cr.teacherLocker.Unlock()
		cr.RemoveTeacher()
	}
}

// 用于删除直播学生的函数，协程不安全
func (cr *chatroom) RemoveStudent(studentId int) {
	if iter := cr.students.Find(studentId); iter.IsValid() {
		stu := iter.Value().(Student)
		stu.SetDelete(true)
		stu.conn.Close()
		stu.conn = nil
		cr.students.Erase(studentId)
	}
}

// 用于删除直播学生的函数，协程安全
func (cr *chatroom) RemoveStudentWithGoroutineSafe(studentId int) {
	if cr.students.Find(studentId).IsValid() {
		cr.students.Find(studentId).Value().(*Student).SetDelete(true)
		cr.studentLocker.Lock()
		cr.RemoveStudent(studentId)
		cr.studentLocker.Unlock()
	}
}

// 用于读取直播教师端消息的函数，协程安全，如果教师已经断开了，那么将会删除这个教师
// 因为原始的读入有一个阻塞，我们这里需要使用时限消除阻塞
func (cr *chatroom) ReadTeacherMessageWithPanicSafe(msg *Message) error {
	defer func() {
		recover()
	}()
	if err := cr.teacher.ReadJSON(msg); err == nil {
		return nil
	} else {
		cr.RemoveTeacherWithGoroutineSafe()
	}
	return errors.New("ReadTeacherMessageWithPanicSafe: ReadFailed")
}

// 用于写直播教师端消息的函数，协程安全，如果教师已经断开了，那么将会删除这个教师
func (cr *chatroom) WriteTeacherMessageWithPanicSafe(msg *Message) error {
	defer func() {
		recover()
	}()
	if err := cr.teacher.WriteJSON(msg); err == nil {
		return nil
	} else {
		cr.RemoveTeacher()
	}
	return errors.New("WriteTeacherMessageWithPanicSafe: WriteFailed")
}

// 用于读取直播学生消息的函数，协程安全，如果学生已经离开了，那么将删除这个学生
func (cr *chatroom) ReadStudentMessageWithPanicSafe(stu *Student, msg *Message) error {
	defer func() {
		recover()
	}()
	if err := stu.conn.ReadJSON(msg); err == nil {
		return nil
	} else {
		cr.RemoveStudentWithGoroutineSafe(stu.GetId())
	}
	return errors.New("ReadStudentMessageWithPanicSafe: ReadFailed")
}

// 用于写直播学生消息的函数，协程安全，如果学生已经离开了，那么将删除这个学生
func (cr *chatroom) WriteStudentMessageWithPanicSafe(stu *Student, msg *Message) error {
	defer func() {
		recover()
	}()
	if err := stu.conn.WriteJSON(msg); err == nil {
		return nil
	} else {
		cr.RemoveStudent(stu.GetId())
	}
	return errors.New("WriteStudentMessageWithPanicSafe: WriteFailed")
}

// 用于监视直播教师端offer，然后进行学生匹配与offer和answer的交换
func (cr *chatroom) OpenOfferAndAnswerChannel() {
	// 主循环
	for {
		// 存储消息结构体
		msg := &Message{}
		// 判断教师是否在线，于在线的情况下进行读取操作
		if err := cr.ReadTeacherMessageWithPanicSafe(msg); err == nil {
			// 寻找一个没有收到offer的学生
			// 为保证线程安全，需要进行上锁（遍历和删除的冲突）
			for iter := cr.students.Begin(); iter.IsValid(); iter.Next() {
				stu := iter.Value().(*Student)
				if !stu.IsOffered() && !stu.IsDelete() {
					// 设置为offered
					stu.SetOffer(true)
					// 启动协程进行offer和answer的链接
					go func(msg *Message) {
						newMsg := &Message{}
						// 传给学生端offer
						if err := cr.WriteStudentMessageWithPanicSafe(stu, msg); err == nil {
							// 从学生端接收answer
							if err := cr.ReadStudentMessageWithPanicSafe(stu, newMsg); err == nil {
								// 传给教师端answer
								if err := cr.WriteTeacherMessageWithPanicSafe(newMsg); err == nil {
									return
								}
							}
						}
						stu.SetOffer(false)
					}(msg)
					break
				}
			}
		}
	}
}

// 设置直播教师端，协程安全，注意这里有对学生端进行lock
func (cr *chatroom) SetTeacher(teacher *websocket.Conn) {
	// 删除原先的教师
	cr.RemoveTeacherWithGoroutineSafe()
	// 设置现在的教师
	cr.teacherLocker.Lock()
	cr.teacher = teacher
	cr.teacherLocker.Unlock()

	// 待发送的消息
	msg := &Message{
		Type: "newPeer",
		// 新增的数目
		Id:   cr.students.Size(),
		Data: nil,
	}
	// 发送消息
	cr.WriteTeacherMessageWithPanicSafe(msg)
}

// 添加直播学生端，线程安全
func (cr *chatroom) AddStudent(stuConn *websocket.Conn) {
	// 生成一个学生端，并添加到映射表中
	cr.studentLocker.Lock()
	stu := &Student{
		stuConn,
		false,
		&sync.RWMutex{},
		cr.inc.CreatId(),
		false,
		&sync.RWMutex{},
	}
	cr.students.Insert(stu.id, stu)
	cr.studentLocker.Unlock()

	// 提醒教师添加一个peer
	cr.WriteTeacherMessageWithPanicSafe(&Message{
		Type: "newPeer",
		Id:   1,
		Data: nil,
	})
}

// 用于删除聊天室中的客户端，协程不安全
func (cr *chatroom) RemoveClient(cli *Client) {
	if iter := cr.chatClient.Find(cli); iter.IsValid() {
		cli.SetDelete(true)
		cli.conn.Close()
		cli.conn = nil
		cr.chatClient.Erase(cli)
	}
}

// 用于删除聊天室中的客户端，协程安全
func (cr *chatroom) RemoveClientWithGoroutineSafe(cli *Client) {
	if iter := cr.chatClient.Find(cli); iter.IsValid() {
		cli.SetDelete(true)
		cr.clientLocker.Lock()
		defer cr.clientLocker.Unlock()
		cr.RemoveClient(cli)
	}
}

// 用于读取聊天室客户端消息，协程安全，如果客户端已经断线，自动删除
func (cr *chatroom) ReadClientMessageWithPanicSafe(cli *Client, msg *Message) error {
	defer func() {
		recover()
	}()
	if err := cli.conn.ReadJSON(msg); err == nil {
		return nil
	} else {
		cr.RemoveClient(cli)
	}
	return errors.New("ReadClientMessageWithPanicSafe: ReadFailed")
}

// 用于聊天室客户端发送消息，协程安全，如果客户端已经断线，自动删除
func (cr *chatroom) WriteClientMessageWithPanicSafe(cli *Client, msg *Message) error {
	defer func() {
		recover()
	}()
	if err := cli.conn.WriteJSON(msg); err == nil {
		return nil
	} else {
		cr.RemoveClient(cli)
	}
	return errors.New("WriteClientMessageWithPanicSafe: WriteFailed")
}

// 用于聊天，监控客户端发送的消息
func (cr *chatroom) ClientStart(cli *Client) {
	defer func() {
		cr.RemoveClientWithGoroutineSafe(cli)
		fmt.Println("a Client exit")
	}()
	fmt.Println("Add a new Client\n", cli)
	for {
		msg := &Message{}
		if err := cr.ReadClientMessageWithPanicSafe(cli, msg); err != nil {
			return
		}
		msg.Id = cli.chatId
		cr.chatMessages <- msg
	}
}

// 用于广播聊天室里的消息
func (cr *chatroom) BroadcastClientMessage() {
	for {
		// 接收消息
		msg := <-cr.chatMessages
		fmt.Println(msg)
		// 进行广播
		// 为保证线程安全，需要进行上锁（遍历和删除的冲突）
		for iter := cr.chatClient.Begin(); iter.IsValid(); iter.Next() {
			cli := iter.Value().(*Client)
			if !cli.IsDelete() && cli.chatId != msg.Id {
				go cr.WriteClientMessageWithPanicSafe(cli, msg)
			}
		}

	}
}

// 添加一个Client，协程安全
func (cr *chatroom) AddClient(cliconn *websocket.Conn) {
	cli := &Client{
		cliconn,
		false,
		&sync.RWMutex{},
		cr.inc.CreatId(),
	}
	// 增加客户端
	cr.clientLocker.Lock()
	cr.chatClient.Insert(cli)
	cr.clientLocker.Unlock()
	// 监视客户端
	go cr.ClientStart(cli)
}

// 用于生成一个Chatroom
func NewChatroom() *chatroom {
	myNewChatroom := &chatroom{
		// ---存储学生端的映射表，int->*Student
		treemap.New(treemap.WithGoroutineSafe()),
		// 用于生成学生端映射表的key
		&IncrementId{&sync.RWMutex{}, 0},
		// 用于删除学生端，使用学生端的互斥锁
		&sync.RWMutex{},
		// ---教师端（只是一个*websocket.Conn）
		nil,
		// 用于删除教师端，使用教师端的互斥锁
		&sync.RWMutex{},
		// 存储聊天客户端的集合 *websocket.Conn
		set.New(set.WithGoroutineSafe()),
		// 用于存储聊天消息的chan，消息队列 *Message
		make(chan *Message, 1000),
		// 用于删除聊天客户端，使用聊天客户端的互斥锁
		&sync.RWMutex{},
	}
	// 开启聊天室广播器
	go myNewChatroom.BroadcastClientMessage()
	// 开启offerAnswer通道
	go myNewChatroom.OpenOfferAndAnswerChannel()
	return myNewChatroom
}
