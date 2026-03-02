// ============================================
// AUTH CHECK
// ============================================
const username = localStorage.getItem('username');
const userID = localStorage.getItem('userID');

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
const chatTitle = document.getElementById('chat-title');
const chatsList = document.getElementById('chats-list');
const currentUserSpan = document.getElementById('current-user');

// Set username in header
currentUserSpan.textContent = '👤 ' + username;
// ============================================
// STATE
// ============================================
let websocket;
let reconnectAttempts = 0;
const maxReconnectAttempts = 5;

let onlineUsers = [];
let currentChat = 'public';

// Store messages per chat
const chatMessages = {
    public: []
};

// Track unread counts
const unreadCounts = {};

// ============================================
// EVENT LISTENERS
// ============================================
sendButton.addEventListener('click', sendMessage);

messageInput.addEventListener('keypress', (event) => {
    if (event.key === 'Enter' && !event.shiftKey) {
        event.preventDefault();
        sendMessage();
    }
});

// ============================================
// WEBSOCKET
// ============================================
function startWebSocket() {
    websocket = new WebSocket('ws://localhost:8080/ws');

    websocket.onopen = () => {
        console.log('WebSocket connected');
        reconnectAttempts = 0;
        websocket.send(username);
        addSystemMessage('public', 'Connected to chat!');
        renderMessages();
    };

    websocket.onmessage = (event) => {
        try {
            const msg = JSON.parse(event.data);
            handleMessage(msg);
        } catch (e) {
            console.log('Raw message:', event.data);
        }
    };

    websocket.onclose = () => {
        console.log('WebSocket closed');
        if (reconnectAttempts < maxReconnectAttempts) {
            reconnectAttempts++;
            const delay = Math.min(1000 * reconnectAttempts, 5000);
            addSystemMessage('public', `Reconnecting in ${delay/1000}s...`);
            renderMessages();
            setTimeout(startWebSocket, delay);
        }
    };

    websocket.onerror = (error) => {
        console.error('WebSocket error:', error);
    };
}

// ============================================
// MESSAGE HANDLER
// ============================================
function handleMessage(msg) {
    switch (msg.type) {
        case 'public':
            addMessage('public', {
                type: 'received',
                from: msg.from,
                content: msg.content,
                timestamp: msg.timestamp
            });
            if (currentChat !== 'public') {
                incrementUnread('public');
            }
            break;

        case 'private':
            ensureChatExists(msg.from);
            addMessage(msg.from, {
                type: 'received',
                from: msg.from,
                content: msg.content,
                timestamp: msg.timestamp
            });
            if (currentChat !== msg.from) {
                incrementUnread(msg.from);
            }
            break;

        case 'private_sent':
            ensureChatExists(msg.to);
            addMessage(msg.to, {
                type: 'sent',
                content: msg.content,
                timestamp: msg.timestamp
            });
            break;

        case 'system':
            addSystemMessage('public', msg.content);
            break;

        case 'user_list':
            onlineUsers = msg.users.filter(u => u !== username);
            renderSidePanel();
            break;
    }
    renderMessages();
}

// ============================================
// CHAT MANAGEMENT
// ============================================
function ensureChatExists(chatKey) {
    if (!chatMessages[chatKey]) {
        chatMessages[chatKey] = [];
    }
}

function addMessage(chatKey, message) {
    ensureChatExists(chatKey);
    chatMessages[chatKey].push(message);
}

function addSystemMessage(chatKey, content) {
    ensureChatExists(chatKey);
    chatMessages[chatKey].push({
        type: 'system',
        content: content,
        timestamp: Date.now()
    });
}

function incrementUnread(chatKey) {
    unreadCounts[chatKey] = (unreadCounts[chatKey] || 0) + 1;
    renderSidePanel();
}

function clearUnread(chatKey) {
    unreadCounts[chatKey] = 0;
    renderSidePanel();
}

// ============================================
// SEND MESSAGE
// ============================================
function sendMessage() {
    const message = messageInput.value.trim();
    if (!message || !websocket || websocket.readyState !== WebSocket.OPEN) return;

    if (currentChat === 'public') {
        // Public message
        websocket.send(message);
        addMessage('public', {
            type: 'sent',
            content: message,
            timestamp: Date.now()
        });
    } else {
        // Private message
        websocket.send(`@${currentChat} ${message}`);
    }

    messageInput.value = '';
    renderMessages();
}

// ============================================
// RENDER SIDE PANEL
// ============================================
function renderSidePanel() {
    chatsList.innerHTML = '';

    // Public chat
    const publicItem = createChatItem('public', '🌐 Public');
    chatsList.appendChild(publicItem);

    // Online users
    onlineUsers.forEach(user => {
        const item = createChatItem(user, user);
        chatsList.appendChild(item);
    });

    // Offline chats (users with messages but not online)
    Object.keys(chatMessages).forEach(chatKey => {
        if (chatKey !== 'public' && !onlineUsers.includes(chatKey)) {
            const item = createChatItem(chatKey, chatKey + ' (offline)');
            chatsList.appendChild(item);
        }
    });
}

function createChatItem(chatKey, displayName) {
    const div = document.createElement('div');
    div.className = 'chat-item' + (currentChat === chatKey ? ' active' : '');
    div.onclick = () => switchChat(chatKey);

    const nameSpan = document.createElement('span');
    nameSpan.className = 'chat-item-name';
    nameSpan.textContent = displayName;
    div.appendChild(nameSpan);

    // Unread dot
    if (unreadCounts[chatKey] > 0) {
        const dot = document.createElement('span');
        dot.className = 'unread-dot';
        div.appendChild(dot);
    }

    return div;
}

// ============================================
// SWITCH CHAT
// ============================================
function switchChat(chatKey) {
    currentChat = chatKey;
    clearUnread(chatKey);

    // Update header
    if (chatKey === 'public') {
        chatTitle.textContent = 'Public';
    } else {
        chatTitle.textContent = chatKey;
    }

    ensureChatExists(chatKey);
    renderSidePanel();
    renderMessages();
}

// ============================================
// RENDER MESSAGES
// ============================================
function renderMessages() {
    chatArea.innerHTML = '';

    const messages = chatMessages[currentChat] || [];

    messages.forEach(msg => {
        const div = document.createElement('div');
        div.className = 'message ' + msg.type;

        if (msg.type === 'system') {
            div.textContent = msg.content;
        } else if (msg.type === 'received') {
            div.innerHTML = `
                <div class="sender">${escapeHtml(msg.from)}</div>
                <div>${escapeHtml(msg.content)}</div>
                <div class="time">${formatTime(msg.timestamp)}</div>
            `;
        } else {
            div.innerHTML = `
                <div>${escapeHtml(msg.content)}</div>
                <div class="time">${formatTime(msg.timestamp)}</div>
            `;
        }

        chatArea.appendChild(div);
    });

    chatArea.scrollTop = chatArea.scrollHeight;
}

// ============================================
// UTILITIES
// ============================================
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

function formatTime(timestamp) {
    const date = new Date(timestamp);
    return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
}

function logout() {
    localStorage.removeItem('username');
    localStorage.removeItem('userID');
    if (websocket) websocket.close();
    window.location.href = '/login.html';
}

// ============================================
// START
// ============================================
document.addEventListener('DOMContentLoaded', () => {
    console.log('Logged in as:', username);
    renderSidePanel();
    startWebSocket();
});