package Server

import (
	ChatroomServer "lionchatRoom/src/chatroomServer"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func Test(c *gin.Context) {
	c.HTML(http.StatusOK, "test.html", gin.H{})
}

var upgrader = websocket.Upgrader{
	// 解决跨域问题
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type tmpS struct {
	Key1 string
	Key2 string
}

func TestWebsocket(c *gin.Context) {
	if ws, err := upgrader.Upgrade(c.Writer, c.Request, nil); err == nil {
		tmp := &tmpS{"fuck", "shit"}
		ws.WriteJSON(tmp)
	}
}

func Server() {
	lionChat := gin.Default()
	// 加载html模板
	lionChat.LoadHTMLGlob("client/html/*")
	// 加载静态资源
	lionChat.Static("client", "./client")
	// chatroomServer路由，即websocket链接
	lionChat.GET("/chatroomServer", ChatroomServer.ChatroomServer)
	// chatroom的POST路由
	lionChat.GET("/chatroom", ChatroomServer.ChatroomPOST)
	// 运行
	lionChat.GET("/test", Test)
	lionChat.GET("/testWebsocket", TestWebsocket)
	lionChat.Run("127.0.0.1:2346")
}
