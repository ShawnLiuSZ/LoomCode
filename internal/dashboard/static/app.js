// LoomCode Dashboard App
// Design Token 驱动 + WebSocket 实时刷新 + 三态覆盖

const API_BASE = '';
let TOKEN = '';
let currentSessions = [];

// ===== Token 提取 =====
function getToken() {
    if (TOKEN) return TOKEN;
    const params = new URLSearchParams(location.search);
    TOKEN = params.get('token') || '';
    return TOKEN;
}

function authURL(path) {
    return `${API_BASE}${path}?token=${getToken()}`;
}

// ===== API 请求（带错误处理） =====
async function apiFetch(path) {
    const resp = await fetch(authURL(path));
    if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
    return resp.json();
}

// ===== 渲染：骨架屏 =====
function renderSkeletons() {
    const list = document.getElementById('session-list');
    list.innerHTML = Array(3).fill(0).map(() =>
        '<li><div class="skeleton skeleton-item"></div></li>'
    ).join('');
}

// ===== 渲染：会话列表（语义化按钮） =====
function renderSessions(sessions) {
    const list = document.getElementById('session-list');
    currentSessions = sessions;

    if (sessions.length === 0) {
        list.innerHTML = '<li class="empty-state">暂无会话</li>';
        return;
    }

    list.innerHTML = sessions.map(s => `
        <li>
            <button class="session-item" data-id="${s.id}" role="option" tabindex="0" aria-label="${s.name}, ${s.messages} messages">
                <span class="session-item__name">${s.name}</span>
                <span class="session-item__meta">${s.messages} messages</span>
            </button>
        </li>
    `).join('');

    list.querySelectorAll('.session-item').forEach(btn => {
        const id = btn.dataset.id;
        btn.addEventListener('click', () => selectSession(id));
        btn.addEventListener('keydown', (e) => {
            if (e.key === 'Enter' || e.key === ' ') {
                e.preventDefault();
                selectSession(id);
            }
        });
    });
}

// ===== 选择会话 =====
function selectSession(id) {
    const session = currentSessions.find(s => s.id === id);
    if (!session) return;

    const details = document.getElementById('session-details');
    details.innerHTML = `
        <h3>${session.name}</h3>
        <p>Messages: ${session.messages}</p>
        <p>ID: ${session.id}</p>
    `;
}

// ===== 渲染：Provider 状态（语义胶囊替代 emoji） =====
function renderStatus(status) {
    const map = { deepseek: 'deepseek-status', mimo: 'mimo-status', openai: 'openai-status' };
    for (const [key, elId] of Object.entries(map)) {
        const el = document.getElementById(elId);
        const state = status[key] || 'idle';
        const connected = state === 'connected';
        const pillClass = connected ? 'pill--ok' : 'pill--down';
        const label = connected ? 'Connected' : 'Disconnected';
        const name = key.charAt(0).toUpperCase() + key.slice(1);
        el.className = `pill ${pillClass}`;
        el.setAttribute('aria-label', `${name}: ${label}`);
        el.innerHTML = `${name}<span class="sr-only">: ${label}</span>`;
    }
}

// ===== 渲染：成本（toFixed(2)） =====
function renderCost(cost) {
    document.getElementById('cost-total').textContent =
        `Cost: $${(cost.total || 0).toFixed(2)} (today: $${(cost.today || 0).toFixed(2)})`;
}

// ===== 渲染：成本折线图 =====
function renderCostChart(history) {
    const canvas = document.getElementById('chart');
    if (!canvas || !history || history.length === 0) return;

    const ctx = canvas.getContext('2d');
    const w = canvas.width = canvas.clientWidth;
    const h = canvas.height = 200;
    const max = Math.max(...history, 0.0001);
    const padding = 20;

    ctx.clearRect(0, 0, w, h);

    // 网格线
    ctx.strokeStyle = 'rgba(255,255,255,0.05)';
    ctx.lineWidth = 1;
    for (let i = 0; i <= 4; i++) {
        const y = padding + (h - 2 * padding) * i / 4;
        ctx.beginPath();
        ctx.moveTo(padding, y);
        ctx.lineTo(w - padding, y);
        ctx.stroke();
    }

    // 折线
    const accent = getComputedStyle(document.documentElement)
        .getPropertyValue('--accent').trim() || '#00d9ff';
    ctx.strokeStyle = accent;
    ctx.lineWidth = 2;
    ctx.beginPath();
    history.forEach((v, i) => {
        const x = padding + (i / Math.max(history.length - 1, 1)) * (w - 2 * padding);
        const y = h - padding - (v / max) * (h - 2 * padding);
        if (i === 0) ctx.moveTo(x, y);
        else ctx.lineTo(x, y);
    });
    ctx.stroke();

    // 数据点
    ctx.fillStyle = accent;
    history.forEach((v, i) => {
        const x = padding + (i / Math.max(history.length - 1, 1)) * (w - 2 * padding);
        const y = h - padding - (v / max) * (h - 2 * padding);
        ctx.beginPath();
        ctx.arc(x, y, 3, 0, Math.PI * 2);
        ctx.fill();
    });
}

// ===== 错误条 =====
function showError(msg, retryFn) {
    const container = document.getElementById('error-container');
    container.innerHTML = `
        <div class="error-bar">
            <span>${msg}</span>
            <button class="error-bar__retry" type="button">Retry</button>
        </div>
    `;
    container.querySelector('.error-bar__retry').addEventListener('click', () => {
        container.innerHTML = '';
        retryFn();
    });
}

// ===== WebSocket 实时刷新 =====
function initWebSocket() {
    const wsURL = `ws://${location.host}/ws?token=${getToken()}`;
    const ws = new WebSocket(wsURL);

    ws.onmessage = (e) => {
        try {
            const { type, payload } = JSON.parse(e.data);
            if (type === 'cost') {
                renderCost(payload);
                renderCostChart(payload.history);
            } else if (type === 'status') {
                renderStatus(payload);
            }
        } catch (err) {
            // 忽略非 JSON 消息
        }
    };

    ws.onclose = () => {
        // 3s 后重连
        setTimeout(() => initWebSocket(), 3000);
    };

    ws.onerror = () => {
        ws.close();
    };
}

// ===== 初始化 =====
async function init() {
    renderSkeletons();

    try {
        const [sessions, cost, status] = await Promise.all([
            apiFetch('/api/sessions'),
            apiFetch('/api/cost'),
            apiFetch('/api/status')
        ]);

        renderSessions(sessions);
        renderCost(cost);
        renderCostChart(cost.history);
        renderStatus(status);
    } catch (err) {
        showError(`加载失败: ${err.message}`, init);
    }

    initWebSocket();
}

// 启动
init();
