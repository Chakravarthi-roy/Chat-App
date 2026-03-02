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
const userInfo = document.getElementById('user-info');
const chatTabs = document.getElementById('chat-tabs');
const onlineUsersList = document.getElementById('online-users-list');

// ============================================
// STATE
// ============================================
let websocket;
let reconnectAttempts = 0;
const maxReconnectAttempts = 5;

let onlineUsers = [];
let currentChat = 'public'; // 'public' or username for private

// Store messages per chat
const chatMessages = {
    public: []
};

// Track unread counts
const unreadCounts = {
    public: 0
};

// ============================================
// DISPLAY USERNAME
// ============================================
if (userInfo) {
    userInfo.textContent = username;
}

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
// WEBSOCKET CONNECTION
// ============================================
function startWebSocket() {
    websocket = new WebSocket('ws://localhost:8080/ws');

    websocket.onopen = () => {
        console.log('WebSocket connected');
        reconnectAttempts = 0;
        websocket.send(username);
        appendSystemMessage('Connected to chat!');
    };

    websocket.onmessage = (event) => {
        try {
            const msg = JSON.parse(event.data);
            handleMessage(msg);
        } catch (e) {
            // Old format message (backwards compatibility)
            console.log('Raw message:', event.data);
        }
    };

    websocket.onclose = (event) => {
        console.log('WebSocket closed');
        
        if (reconnectAttempts < maxReconnectAttempts) {
            reconnectAttempts++;
            const delay = Math.min(1000 * reconnectAttempts, 5000);
            appendSystemMessage(`Disconnected. Reconnecting in ${delay/1000}s...`);
            setTimeout(startWebSocket, delay);
        } else {
            appendSystemMessage('Connection lost. Please refresh the page.');
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
            handlePublicMessage(msg);
            break;
        case 'private':
            handlePrivateMessage(msg);
            break;
        case 'private_sent':
            handlePrivateSent(msg);
            break;
        case 'system':
            appendSystemMessage(msg.content);
            break;
        case 'user_list':
            updateUserList(msg.users);
            break;
        default:
            console.log('Unknown message type:', msg);
    }
}

// ============================================
// PUBLIC MESSAGES
// ============================================
function handlePublicMessage(msg) {
    // Store message
    if (!chatMessages.public) chatMessages.public = [];
    chatMessages.public.push(msg);

    // If not viewing public chat, increment unread
    if (currentChat !== 'public') {
        unreadCounts.public = (unreadCounts.public || 0) + 1;
        updateTabUnread('public', unreadCounts.public);
    } else {
        // Display message
        appendReceivedMessage(msg.from, msg.content, msg.timestamp);
    }
}

// ============================================
// PRIVATE MESSAGES
// ============================================
function handlePrivateMessage(msg) {
    const chatKey = msg.from;

    // Ensure chat exists
    if (!chatMessages[chatKey]) {
        chatMessages[chatKey] = [];
        createPrivateChatTab(chatKey);
    }

    // Store message
    chatMessages[chatKey].push({
        ...msg,
        received: true
    });

    // If not viewing this chat, increment unread
    if (currentChat !== chatKey) {
        unreadCounts[chatKey] = (unreadCounts[chatKey] || 0) + 1;
        updateTabUnread(chatKey, unreadCounts[chatKey]);
        updateUserUnread(chatKey, unreadCounts[chatKey]);
    } else {
        // Display message
        appendPrivateMessage(msg.from, msg.content, msg.timestamp, true);
    }
}

function handlePrivateSent(msg) {
    const chatKey = msg.to;

    // Ensure chat exists
    if (!chatMessages[chatKey]) {
        chatMessages[chatKey] = [];
        createPrivateChatTab(chatKey);
    }

    // Store message
    chatMessages[chatKey].push({
        ...msg,
        received: false
    });

    // If viewing this chat, display
    if (currentChat === chatKey) {
        appendPrivateMessage(msg.to, msg.content, msg.timestamp, false);
    }
}

// ============================================
// SEND MESSAGE
// ============================================
function sendMessage() {
    const message = messageInput.value.trim();
    if (!message || !websocket || websocket.readyState !== WebSocket.OPEN) return;

    // If in private chat, auto-add @username prefix
    if (currentChat !== 'public' && !message.startsWith('@')) {
        websocket.send(`@${currentChat} ${message}`);
    } else if (currentChat === 'public') {
        websocket.send(message);
        // Show own message immediately for public chat
        appendSentMessage(message);
    } else {
        websocket.send(message);
    }

    messageInput.value = '';
}

// ============================================
// APPEND MESSAGES
// ============================================
function appendSentMessage(content) {
    const div = document.createElement('div');
    div.className = 'message sent';
    div.innerHTML = `
        <div class="message-content">${escapeHtml(content)}</div>
        <div class="message-meta">${formatTime(Date.now())}</div>
    `;
    chatArea.appendChild(div);
    chatArea.scrollTop = chatArea.scrollHeight;

    // Store in public messages
    chatMessages.public.push({
        type: 'public',
        from: username,
        content: content,
        timestamp: Date.now(),
        sent: true
    });
}

function appendReceivedMessage(from, content, timestamp) {
    const div = document.createElement('div');
    div.className = 'message received';
    div.innerHTML = `
        <div class="sender-name">${escapeHtml(from)}</div>
        <div class="message-content">${escapeHtml(content)}</div>
        <div class="message-meta">${formatTime(timestamp)}</div>
    `;
    chatArea.appendChild(div);
    chatArea.scrollTop = chatArea.scrollHeight;
}

function appendPrivateMessage(otherUser, content, timestamp, isReceived) {
    const div = document.createElement('div');
    div.className = `message private ${isReceived ? 'received' : 'sent'}`;
    
    if (isReceived) {
        div.innerHTML = `
            <div class="private-label">🔒 Private</div>
            <div class="sender-name">${escapeHtml(otherUser)}</div>
            <div class="message-content">${escapeHtml(content)}</div>
            <div class="message-meta">${formatTime(timestamp)}</div>
        `;
    } else {
        div.innerHTML = `
            <div class="private-label">🔒 To: ${escapeHtml(otherUser)}</div>
            <div class="message-content">${escapeHtml(content)}</div>
            <div class="message-meta">${formatTime(timestamp)}</div>
        `;
    }
    
    chatArea.appendChild(div);
    chatArea.scrollTop = chatArea.scrollHeight;
}

function appendSystemMessage(content) {
    const div = document.createElement('div');
    div.className = 'message system';
    div.textContent = content;
    chatArea.appendChild(div);
    chatArea.scrollTop = chatArea.scrollHeight;
}

// ============================================
// USER LIST
// ============================================
function updateUserList(users) {
    onlineUsers = users.filter(u => u !== username);
    renderUserList();
}

function renderUserList() {
    onlineUsersList.innerHTML = '';

    onlineUsers.forEach(user => {
        const div = document.createElement('div');
        div.className = 'user-item';
        div.onclick = () => openPrivateChat(user);

        const unread = unreadCounts[user] || 0;

        div.innerHTML = `
            <span class="user-status online"></span>
            <span class="user-name">${escapeHtml(user)}</span>
            ${unread > 0 ? `<span class="user-unread">${unread}</span>` : ''}
        `;

        onlineUsersList.appendChild(div);
    });

    if (onlineUsers.length === 0) {
        onlineUsersList.innerHTML = '<div style="color: #999; font-size: 0.85em;">No other users online</div>';
    }
}

function updateUserUnread(user, count) {
    renderUserList(); // Re-render to update badge
}

// ============================================
// CHAT TABS
// ============================================
function createPrivateChatTab(user) {
    // Check if tab already exists
    if (document.querySelector(`.chat-tab[data-chat="${user}"]`)) return;

    const tab = document.createElement('button');
    tab.className = 'chat-tab';
    tab.setAttribute('data-chat', user);
    tab.innerHTML = `
        🔒 ${escapeHtml(user)}
        <span class="unread-badge" style="display: none;">0</span>
        <span class="close-tab" onclick="closeTab(event, '${user}')">×</span>
    `;
    tab.onclick = (e) => {
        if (!e.target.classList.contains('close-tab')) {
            switchChat(user);
        }
    };

    chatTabs.appendChild(tab);
}

function switchChat(chatKey) {
    // Update active tab
    document.querySelectorAll('.chat-tab').forEach(tab => {
        tab.classList.remove('active');
        if (tab.getAttribute('data-chat') === chatKey) {
            tab.classList.add('active');
        }
    });

    // Clear unread
    unreadCounts[chatKey] = 0;
    updateTabUnread(chatKey, 0);
    if (chatKey !== 'public') {
        updateUserUnread(chatKey, 0);
    }

    // Update current chat
    currentChat = chatKey;

    // Render messages for this chat
    renderChatMessages(chatKey);
}

function renderChatMessages(chatKey) {
    chatArea.innerHTML = '';

    const messages = chatMessages[chatKey] || [];

    messages.forEach(msg => {
        if (msg.type === 'system') {
            appendSystemMessage(msg.content);
        } else if (msg.type === 'public') {
            if (msg.sent || msg.from === username) {
                appendSentMessage(msg.content);
            } else {
                appendReceivedMessage(msg.from, msg.content, msg.timestamp);
            }
        } else if (msg.type === 'private' || msg.type === 'private_sent') {
            const isReceived = msg.received || msg.type === 'private';
            const otherUser = isReceived ? msg.from : msg.to;
            appendPrivateMessage(otherUser, msg.content, msg.timestamp, isReceived);
        }
    });
}

function updateTabUnread(chatKey, count) {
    const tab = document.querySelector(`.chat-tab[data-chat="${chatKey}"]`);
    if (!tab) return;

    const badge = tab.querySelector('.unread-badge');
    if (badge) {
        if (count > 0) {
            badge.textContent = count;
            badge.style.display = 'inline';
        } else {
            badge.style.display = 'none';
        }
    }
}

function closeTab(event, chatKey) {
    event.stopPropagation();
    
    // Remove tab
    const tab = document.querySelector(`.chat-tab[data-chat="${chatKey}"]`);
    if (tab) tab.remove();

    // Clear messages and unread
    delete chatMessages[chatKey];
    delete unreadCounts[chatKey];

    // Switch to public if closing current chat
    if (currentChat === chatKey) {
        switchChat('public');
    }
}

function openPrivateChat(user) {
    if (!chatMessages[user]) {
        chatMessages[user] = [];
    }
    createPrivateChatTab(user);
    switchChat(user);
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
// INITIALIZE PUBLIC TAB CLICK
// ============================================
document.querySelector('.chat-tab[data-chat="public"]').onclick = () => switchChat('public');

// ============================================
// START APP
// ============================================
document.addEventListener('DOMContentLoaded', () => {
    console.log('Logged in as:', username);
    startWebSocket();
});