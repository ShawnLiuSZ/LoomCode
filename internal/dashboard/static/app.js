// LoomCode Dashboard App

// API 基础路径
const API_BASE = '';

// 获取会话列表
async function fetchSessions() {
    try {
        const response = await fetch(`${API_BASE}/api/sessions`);
        return await response.json();
    } catch (error) {
        console.error('Failed to fetch sessions:', error);
        return [];
    }
}

// 获取成本统计
async function fetchCost() {
    try {
        const response = await fetch(`${API_BASE}/api/cost`);
        return await response.json();
    } catch (error) {
        console.error('Failed to fetch cost:', error);
        return { total: 0, today: 0, history: [] };
    }
}

// 获取 Provider 状态
async function fetchStatus() {
    try {
        const response = await fetch(`${API_BASE}/api/status`);
        return await response.json();
    } catch (error) {
        console.error('Failed to fetch status:', error);
        return {};
    }
}

// 渲染会话列表
function renderSessions(sessions) {
    const list = document.getElementById('session-list');
    list.innerHTML = sessions.map(s => `
        <li data-id="${s.id}">
            <strong>${s.name}</strong>
            <span>${s.messages} messages</span>
        </li>
    `).join('');

    // 添加点击事件
    list.querySelectorAll('li').forEach(li => {
        li.addEventListener('click', () => {
            const id = li.dataset.id;
            selectSession(id, sessions);
        });
    });
}

// 选择会话
function selectSession(id, sessions) {
    const session = sessions.find(s => s.id === id);
    if (!session) return;

    const details = document.getElementById('session-details');
    details.innerHTML = `
        <h3>${session.name}</h3>
        <p>Messages: ${session.messages}</p>
        <p>ID: ${session.id}</p>
    `;
}

// 渲染 Provider 状态
function renderStatus(status) {
    document.getElementById('deepseek-status').textContent =
        `DeepSeek: ${status.deepseek === 'connected' ? '✅' : '❌'}`;
    document.getElementById('mimo-status').textContent =
        `MiMo: ${status.mimo === 'connected' ? '✅' : '❌'}`;
    document.getElementById('openai-status').textContent =
        `OpenAI: ${status.openai === 'connected' ? '✅' : '❌'}`;
}

// 渲染成本
function renderCost(cost) {
    document.getElementById('cost-total').textContent =
        `Cost: $${cost.total.toFixed(4)}`;
}

// 初始化
async function init() {
    const [sessions, cost, status] = await Promise.all([
        fetchSessions(),
        fetchCost(),
        fetchStatus()
    ]);

    renderSessions(sessions);
    renderCost(cost);
    renderStatus(status);
}

// 启动
init();
