# lionchatRoom
网课聊天室

目前教师可以直播，学生不可以直播

教师学生可以语言聊天

利用webrtc实现直播，websocket实现聊天和沟通
后端用go语言写的

依赖有gin和liyue201/gostl以及gorilla/websocket

run起来后
url标准：
http://127.0.0.1:2346/chatroom?who=[teacher/student]&username=[]

一个教师，多个学生

![截图](./lionchatRoom/doc/img/%E6%88%AA%E5%9B%BE.png)
