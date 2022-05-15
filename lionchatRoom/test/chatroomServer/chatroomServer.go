package chatroomServer

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var myChatroom = NewChatroom()
var upgrader = websocket.Upgrader{
	// 解决跨域问题
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func ChatroomServer(c *gin.Context) {
	if ws, err := upgrader.Upgrade(c.Writer, c.Request, nil); err == nil {
		message := Message{}
		for {
			if err := ws.ReadJSON(&message); err == nil {
				switch message.Type {
				case "webrtcSignal":
					{
						switch message.Data["who"] {
						case "teacher":
							{
								go myChatroom.SetTeacher(ws)
							}
						case "student":
							{
								go myChatroom.AddStudent(ws)
							}
						}
					}
				case "chatSignal":
					{
						go myChatroom.AddChatClient(ws)
					}
				}
				break
			} else {
				log.Fatal(err)
			}
		}
	} else {
		log.Fatal("websocket链接错误", err)
	}
}

func ChatroomPOST(c *gin.Context) {
	c.HTML(http.StatusOK, "chatroom.html", gin.H{
		"roomname": "LionchatRoom",
	})

}
