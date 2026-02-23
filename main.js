// ============================================
// AUTH CHECK - Get username from localStorage
// ============================================
const username = localStorage.getItem('username');
const userID = localStorage.getItem('userID');

// Redirect to login if not authenticated
if (!username) {
    alert('Please login first!');
    window.location.href = '/login.html';
    throw new Error('Not authenticated');
}

// ============================================
// DOM ELEMENTS
// ============================================
const chatArea = document.getElementById('chat-area');
const messageInput = document.getElementById('message-input');
const sendButton = document.getElementById('send-button');

// ============================================
// WEBSOCKET VARIABLES
// ============================================
let websocket;
let reconnectAttempts = 0;
const maxReconnectAttempts = 5;

// ============================================
// EVENT LISTENERS (outside function to prevent duplicates)
// ============================================
sendButton.addEventListener('click', sendMessage);

messageInput.addEventListener('keypress', (event) => {
    if (event.key === 'Enter' && !event.shiftKey) {
        event.preventDefault();
        sendMessage();
    }
});

// ============================================
// HELPER FUNCTIONS
// ============================================
function sendMessage() {
    const message = messageInput.value.trim();
    if (message && websocket && websocket.readyState === WebSocket.OPEN) {
        websocket.send(message);
        appendMessage('You: ' + message, 'sent');
        messageInput.value = '';
    }
}

function appendMessage(text, type) {
    const messageElement = document.createElement('p');
    messageElement.textContent = text;
    messageElement.classList.add(type);
    chatArea.appendChild(messageElement);
    chatArea.scrollTop = chatArea.scrollHeight;
}

function logout() {
    localStorage.removeItem('username');
    localStorage.removeItem('userID');
    if (websocket) {
        websocket.close();
    }
    window.location.href = '/login.html';
}

// ============================================
// WEBSOCKET CONNECTION
// ============================================
function startWebSocket() {
    websocket = new WebSocket('ws://localhost:8080/ws');

    websocket.onopen = () => {
        console.log('WebSocket connection established.');
        reconnectAttempts = 0;
        websocket.send(username);
        appendMessage('Connected to chat!', 'system');
    };

    websocket.onmessage = (event) => {
        const message = event.data;
        appendMessage(message, 'received');
    };

    websocket.onclose = (event) => {
        console.log('WebSocket connection closed.', event.code, event.reason);
        
        // Auto-reconnect logic
        if (reconnectAttempts < maxReconnectAttempts) {
            reconnectAttempts++;
            const delay = Math.min(1000 * reconnectAttempts, 5000);
            appendMessage(`Disconnected. Reconnecting in ${delay/1000}s... (attempt ${reconnectAttempts})`, 'system');
            setTimeout(startWebSocket, delay);
        } else {
            appendMessage('Connection lost. Please refresh the page.', 'system');
        }
    };

    websocket.onerror = (error) => {
        console.error('WebSocket error:', error);
    };
}

// ============================================
// START APP
// ============================================
document.addEventListener('DOMContentLoaded', () => {
    console.log('Logged in as:', username);
    startWebSocket();
});