ws  = new WebSocket('ws://127.0.0.1:2346/testWebsocket')

ws.onopen = function(evt){
    console.log("Connected ..")
}

ws.onmessage = function(evt){
    console.log(evt.data)
}

ws.onclose = function(evt){
    console.log("closed")
}