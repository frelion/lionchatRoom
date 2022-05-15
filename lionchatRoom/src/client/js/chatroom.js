$(document).ready(function () {
    if (getUrlParam("who")=='student'){
        $('#play').hide()
    }
    $('#sendButton').on('click', function () {
        sendMsg()
    })
    $(function () {
        $('#msg-text').bind('keypress', function (event) {
            if (event.keyCode == "13") {
                event.preventDefault();
                sendMsg();
            }
        });
    })
    $('#play').on('click', function () {
        if ($('#play').text() == "开始直播") {
            if (who == "student"){
                who = "teacher"
                // 请求成为老师
                ws.send({
                    Type:"beTeacher",
                    Id:-1,
                    Data:nil,
                })
            }
            navigator.mediaDevices.getUserMedia({
                video: true,
                audio: true
            }).then(addStream).catch(() => { })
            $('#play').text("暂停直播")
            isAlive = true
        } else {
            mediaStreamTrack.getTracks().forEach(function (track) {
                track.stop();
            });
            $('#play').text("开始直播")
            isAlive = false
        }
    })
})

//who?
who = getUrlParam('who')

// 用于直播
var ws = new WebSocket('ws://127.0.0.1:2346/chatroomServer');
mediaStreamTrack = null
var peers = []
var isAlive = false
dock = -1
// websocket WEBRTC 配置
ws.onopen = function (evt) {
    console.log('Connection open ..');
    peers.push(newPeer(peers.length))
    // 表明当前身份
    ws.send(JSON.stringify({
        Type: "webrtcSignal", Data: {
            "who": who
        }
    }))
};

ws.onmessage = function (evt) {
    jsonMessage = $.parseJSON(evt.data)
    // console.log(jsonMessage)
    if (jsonMessage.Type == "webrtc") {
        // console.log(jsonMessage)
        // 如果是教师端，此刻接收的是一个answer
        // 如果是学生端，此刻接收的是一个offer
        if (who == "student") {
            dock = jsonMessage.Id
            peers[0].signal(jsonMessage.Data)
        } else {
            // jsonMessage.Id 是指明的
            peers[jsonMessage.Id].signal(jsonMessage.Data)
        }

    } else if (jsonMessage.Type == "newPeer") {
        // 表示多了一个学生，那么新增一个peer
        for (i=1;i<=jsonMessage.Id;i++){
            peer = newPeer(peers.length)
            peers.push(peer)
            if (isAlive) {
                // 如果此刻已经是直播状态了那么，发送offer
                peer.initiator = true
                peer.addStream(mediaStreamTrack)
            }
        }
    } else if (jsonMessage.Type == "deletePeer"){
        for (i=1;i<=jsonMessage.Id;i++){
            peers.pop()
        }
    } else if (jsonMessage.Type == "beStudent"){
        who = "student"
        isAlive = false
        dock = -1
        while (peers.length>1){
            peers.pop()
        }
    }
};

ws.onclose = function (evt) {
    console.log('Connection closed.');
};
// mediaStreamTrack
function addStream(stream) {
    mediaStreamTrack = stream
    video = $("#video")[0]
    if ('srcObject' in video) {
        video.srcObject = stream
    } else {
        video.src = window.URL.createObjectURL(stream) // 老式浏览器
    }
    video.play()
    for (i = 1; i < peers.length; i++) {
        peers[i].initiator = true
        peers[i].addStream(stream)
    }
}
// 生成新的peer
function newPeer(id) {
    peer = new SimplePeer({
        initiator: false,
        trickle: false,
    })
    peer.on('error', data => {
        console.log("data:", data)
    })
    peer.on('signal', data => {
        if (who == "student") {
            // send answer
            ws.send(JSON.stringify({ Type: "webrtc", Id: dock, Data: data }))
        }else{
            // send offer
            ws.send(JSON.stringify({ Type: "webrtc", Id: id, Data: data }))
        }
    })
    peer.on('stream', stream => {
        video = $("#video")[0]
        if ('srcObject' in video) {
            video.srcObject = stream
        } else {
            video.src = window.URL.createObjectURL(stream) // for older browsers
        }
        video.play()
    })
    return peer
}


// 聊天
chatws = new WebSocket('ws://127.0.0.1:2346/chatroomServer');
// 用于聊天的websocket的配置
chatws.onopen = function (evt) {
    console.log('chatWebsocket Connection open ..');
    // 表明当前身份
    chatws.send(JSON.stringify({
        Type: "chatSignal", Data: {
            "who": who
        }
    }))
};

chatws.onmessage = function (evt) {
    jsonMessage = $.parseJSON(evt.data)
    reply(jsonMessage.Data)
};

chatws.onclose = function (evt) {
    console.log('chatWebsocket Connection closed.');
};
function sendMsg() {
    myMsg = $("#msg-text").val()
    if (!myMsg) {
        return;
    }
    var scrollHeight = 0;
    var str = getMyMsg(myMsg)
    $("#bottom-link").before(str);
    scrollHeight = $('.talk-list-div').prop("scrollHeight");
    $('.talk-list-div').scrollTop(scrollHeight, 200);
    $("#msg-text").val("").focus();
    chatws.send(JSON.stringify({
        Type : "chatMessage",
        Data : {
            username : getUrlParam('username'),
            message : myMsg
        }
    }))
}
function reply(replyMsg) {
    var str = getReplyMsg(replyMsg);
    $("#bottom-link").before(str);
    scrollHeight = $('.talk-list-div').prop("scrollHeight");
    $('.talk-list-div').scrollTop(scrollHeight, 200);
};
function getMyMsg(myMsg) {
    console.log(decodeURI(escape(getUrlParam('username'))))
    var str = '<div class="msg-div">';
    str += '<div class="my-msg-div">';
    str += '<div class="header-img-div">';
    str += '<button>' + decodeURI(escape(getUrlParam('username'))) + '</button>'
    str += '</div>';
    str += '<div class="msg-content">';
    str += myMsg;
    str += '</div>';
    str += '</div>';
    str += '</div>';
    return str
}
function getReplyMsg(replyMsg) {
    var str = '<div class="msg-div">';
    str += '<div class="other-msg-div">';
    str += '<div class="header-img-div">';
    // str += '<img src="http://www.jq22.com/img/cs/500x500-2.png" />';
    str += '<p>' + replyMsg.username + '</p>'
    str += '</div>';
    str += '<div class="msg-content">';
    str += replyMsg.message;
    str += '</div>';
    str += '</div>';
    str += '</div>';
    return str
}

function getUrlParam(name) {

    var reg = new RegExp("(^|&)" + name + "=([^&]*)(&|$)"); //构造一个含有目标参数的正则表达式对象

    var r = window.location.search.substr(1).match(reg);  //匹配目标参数

    if (r != null) return unescape(r[2]); return null; //返回参数值

}
