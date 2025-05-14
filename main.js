const chatArea = document.getElementById('chat-area');
const messageInput = document.getElementById('message-input');
const sendButton = document.getElementById('send-button');
// const websocket = new WebSocket('ws://localhost:8080/ws');
let websocket; //connected only when user is logged in! until then, stays a variable

document.addEventListener('DOMContentLoaded', () => {
    startWebSocket();
    // const storedUsername = prompt('Enter your user/nickname:')
    // if(storedUsername) {
    //     username = storedUsername
    //     startWebSocket();
    // } else {
    //     startWebSocket();
    // }
});

//now the websocket function
function startWebSocket() {
    websocket = new WebSocket('ws://localhost:8080/ws');

    //here goes all the functions related to websocket
    websocket.onopen = () => {
        console.log('WebSocket connection established.')
        websocket.send(username)
    };
    
    websocket.onmessage = (event) => {
        const message = event.data
        const messageElement = document.createElement('p');
        messageElement.textContent = 'Received: '+ message;
        chatArea.appendChild(messageElement);
        chatArea.scrollTop = chatArea.scrollHeight 
    };
    
    websocket.onclose = () => {
        console.log('WebSocket connection closed.')
    };
    
    websocket.onerror = (error) => {
        console.error('WebSocket error:', error)
    };
    
    sendButton.addEventListener('click', () => {
        const message = messageInput.value
        if(message) {
            websocket.send(message)
            const messageElement = document.createElement('p');
            messageElement.textContent = 'Sent: '+ message;
            chatArea.appendChild(messageElement);
            messageInput.value = ''
            chatArea.scrollTop = chatArea.scrollHeight
        }
    });
    
    messageInput.addEventListener('keypress', (event) => {
        if(event.key === 'Enter' && !event.shiftKey) {
            sendButton.click();
            event.preventDefault();
        }
    });
}

