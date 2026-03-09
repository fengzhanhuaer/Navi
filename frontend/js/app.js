/**
 * app.js — 个人导航页主逻辑（含认证流程）
 */

// ── 应用状态 ──────────────────────────────────
const state = {
    groups: [],
    sites: [],
    settings: {},
};

// ── DOM 引用 ──────────────────────────────────
const $ = id => document.getElementById(id);

const DOM = {
    appRoot: $('appRoot'),
    greeting: $('greeting'),
    searchEngineSelector: $('searchEngineSelector'),
    currentEngineBtn: $('currentEngineBtn'),
    currentEngineIcon: $('currentEngineIcon'),
    engineDropdown: $('engineDropdown'),
    searchInput: $('searchInput'),
    searchBtn: $('searchBtn'),
    bookmarksSection: $('bookmarksSection'),
    btnTheme: $('btnTheme'),
    iconSun: $('iconSun'),
    iconMoon: $('iconMoon'),
    btnSettings: $('btnSettings'),
    btnLogout: $('btnLogout'),
    toastContainer: $('toastContainer'),

    // Settings modal (hidden placeholder elements)
    settingsModalBackdrop: $('settingsModalBackdrop'),
    settingBackground: $('settingBackground'),
    settingSearchEngine: $('settingSearchEngine'),
    btnUpgrade: $('btnUpgrade'),
    upgradeLog: $('upgradeLog'),
};

// ── Toast 通知 ────────────────────────────────
function toast(msg, type = 'info', duration = 3000) {
    const el = document.createElement('div');
    el.className = `toast ${type}`;
    el.textContent = msg;
    DOM.toastContainer.appendChild(el);
    setTimeout(() => {
        el.style.opacity = '0';
        el.style.transform = 'translateX(20px)';
        el.style.transition = 'all .3s ease';
        setTimeout(() => el.remove(), 300);
    }, duration);
}

function showApp() {
    DOM.appRoot.classList.remove('hidden');
}

// ── 退出登录 ──────────────────────────────────
DOM.btnLogout.addEventListener('click', () => {
    if (!confirm('确定退出登录？')) return;
    localStorage.removeItem('navi_token');
    location.replace('/login');
});

// ── 问候语 ────────────────────────────────────
function updateGreeting() {
    const h = new Date().getHours();
    // [text, emoji] 分开存，方便单独渲染 emoji（避免 background-clip:text 把 emoji 裁成透明）
    const greetings = [
        [5,  '凌晨好，夜猫子', '🌙'],
        [9,  '早上好！', '☀️'],
        [12, '上午好！', '🌤'],
        [14, '中午好！', '🍱'],
        [18, '下午好！', '🍵'],
        [22, '晚上好！', '🌙'],
        [24, '深夜了，注意休息', '🌛'],
    ];
    const g = greetings.find(([limit]) => h < limit) || greetings[greetings.length - 1];
    // emoji 用独立 span，CSS 会重置 -webkit-text-fill-color 让其正常渲染
    DOM.greeting.innerHTML = `${g[1]}<span class="greeting-emoji">${g[2]}</span>`;
}

// ── 主题 ──────────────────────────────────────
function applyTheme(theme) {
    document.body.classList.toggle('dark', theme === 'dark');
    document.body.classList.toggle('light', theme === 'light');
    if (DOM.iconSun && DOM.iconMoon) {
        DOM.iconSun.classList.toggle('hidden', theme === 'dark');
        DOM.iconMoon.classList.toggle('hidden', theme !== 'dark');
    }
}

DOM.btnTheme.addEventListener('click', async () => {
    const newTheme = document.body.classList.contains('dark') ? 'light' : 'dark';
    applyTheme(newTheme);
    state.settings.theme = newTheme;
    try { await api.settings.set('theme', newTheme); } catch { }
});

// ── Favicon ───────────────────────────────────
function getFaviconUrl(siteUrl) {
    return `/api/favicon?url=${encodeURIComponent(siteUrl)}`;
}



// ── 搜索引擎切换 ────────────────────────────────
const ENGINES = {
    google: { url: 'https://www.google.com/search?q=', icon: 'https://www.google.com/s2/favicons?domain=google.com&sz=32', name: 'Google' },
    bing: { url: 'https://www.bing.com/search?q=', icon: 'https://www.google.com/s2/favicons?domain=bing.com&sz=32', name: 'Bing' },
    baidu: { url: 'https://www.baidu.com/s?wd=', icon: 'https://www.google.com/s2/favicons?domain=baidu.com&sz=32', name: 'Baidu' }
};

let currentEngine = localStorage.getItem('navi_search_engine') || 'google';
if (!ENGINES[currentEngine]) currentEngine = 'google';

function setEngine(engineKey) {
    currentEngine = engineKey;
    localStorage.setItem('navi_search_engine', engineKey);
    if (DOM.currentEngineIcon) {
        DOM.currentEngineIcon.src = ENGINES[engineKey].icon;
        DOM.currentEngineIcon.alt = ENGINES[engineKey].name;
    }
    if (DOM.engineDropdown) DOM.engineDropdown.classList.add('hidden');
    DOM.searchInput.focus();
}

setEngine(currentEngine);

if (DOM.currentEngineBtn) {
    DOM.currentEngineBtn.addEventListener('click', (e) => {
        e.stopPropagation();
        DOM.engineDropdown.classList.toggle('hidden');
    });
}

if (DOM.engineDropdown) {
    DOM.engineDropdown.querySelectorAll('.engine-option').forEach(opt => {
        opt.addEventListener('click', (e) => {
            e.stopPropagation();
            setEngine(opt.dataset.engine);
        });
    });
}

document.addEventListener('click', e => {
    if (DOM.searchEngineSelector && !DOM.searchEngineSelector.contains(e.target)) {
        if (DOM.engineDropdown) DOM.engineDropdown.classList.add('hidden');
    }
});

// ── 搜索 ──────────────────────────────────────
function doSearch() {
    const q = DOM.searchInput.value.trim();
    if (!q) return;
    if (/^https?:\/\//i.test(q) || /^[\w-]+\.\w{2,}/.test(q)) {
        window.open(/^https?:\/\//i.test(q) ? q : 'https://' + q, '_blank');
        return;
    }
    window.open(ENGINES[currentEngine].url + encodeURIComponent(q), '_blank');
}

DOM.searchBtn.addEventListener('click', doSearch);
DOM.searchInput.addEventListener('keydown', e => e.key === 'Enter' && doSearch());

document.addEventListener('keydown', e => {
    if (e.key === '/' && document.activeElement !== DOM.searchInput) {
        e.preventDefault();
        DOM.searchInput.focus();
    }
});

// ── 书签渲染（Tab 形式，只读，无操作按钮）─────────────────
function renderBookmarks() {
    if (!state.groups.length) {
        DOM.bookmarksSection.innerHTML = `
      <p style="text-align:center;color:var(--text-muted);padding:40px;">
        还没有任何分组，<a href="/edit" style="color:var(--accent-1);text-decoration:none;">点击这里</a> 开始添加吧！
      </p>`;
        return;
    }

    // Default to the first group if no active tab is selected, or if the active tab is invalid
    if (state.activeTabId == null || !state.groups.find(g => g.id === state.activeTabId)) {
        state.activeTabId = state.groups[0].id;
    }

    // 1. Render Tabs Navigation
    const tabsNavHtml = `
      <div class="tabs-nav blur-glass">
        ${state.groups.map(group => `
          <button class="tab-btn ${group.id === state.activeTabId ? 'active' : ''}" data-tab-id="${group.id}">
            <span class="tab-icon">${group.icon || '📁'}</span>
            <span class="tab-name">${group.name}</span>
          </button>
        `).join('')}
      </div>
    `;

    // 2. Render Active Tab Content
    const activeGroupSites = state.sites.filter(s => s.group_id === state.activeTabId);
    let sitesHtml = '';

    if (activeGroupSites.length === 0) {
        sitesHtml = `
          <div style="text-align:center;color:var(--text-muted);padding:40px 0;width:100%;">
            该分组下还没有书签
          </div>`;
    } else {
        sitesHtml = activeGroupSites.map(site => {
            const isEmoji = site.icon && !site.icon.startsWith('http') && !site.icon.startsWith('/');
            const faviconEl = isEmoji
                ? site.icon
                : `<img src="${getFaviconUrl(site.url)}" alt="${site.title}" loading="lazy" onerror="this.parentElement.textContent='🌐'" />`;
            return `
        <a class="site-card" href="${site.url}" target="_blank" data-site-id="${site.id}">
          <div class="site-favicon">${faviconEl}</div>
          <span class="site-title">${site.title}</span>
        </a>`;
        }).join('');
    }

    const tabsContentHtml = `
      <div class="tabs-content">
        <div class="sites-grid">
          ${sitesHtml}
        </div>
      </div>
    `;

    DOM.bookmarksSection.innerHTML = `
      <div class="tabs-container">
        ${tabsNavHtml}
        ${tabsContentHtml}
      </div>
    `;

    bindTabEvents();
}

function bindTabEvents() {
    DOM.bookmarksSection.querySelectorAll('.tab-btn').forEach(btn => {
        btn.addEventListener('click', (e) => {
            const tabId = parseInt(btn.dataset.tabId);
            if (state.activeTabId !== tabId) {
                state.activeTabId = tabId;
                renderBookmarks(); // Re-render to switch tabs
            }
        });
    });
}

// ── 背景 ──────────────────────────────────────
function applyBackground(bg) { document.body.dataset.bg = bg; }

// ── 加载主应用数据 ────────────────────────────
async function loadApp() {
    showApp();
    updateGreeting();
    setInterval(updateGreeting, 60_000);
    try {
        const data = await api.allData();
        state.settings = data.settings || {};
        state.groups = data.groups || [];
        state.sites = data.sites || [];
        applyTheme(state.settings.theme || 'dark');
        applyBackground(state.settings.background || 'gradient');
        renderBookmarks();
    } catch (err) {
        console.error('Load failed:', err);
        toast('加载数据失败，请刷新重试', 'error', 6000);
        renderBookmarks();
    }
}

// ── 初始化入口 ────────────────────────────────
async function init() {
    const token = localStorage.getItem('navi_token');

    try {
        const { registered } = await api.auth.status();

        if (token) {
            try {
                showApp();
                await loadApp();
            } catch (appErr) {
                if (appErr.message !== '请重新登录') {
                    console.error('App init failed:', appErr);
                    toast('加载失败：' + appErr.message, 'error');
                } else {
                    location.replace('/login');
                }
            }
        } else if (!registered) {
            location.replace('/register');
        } else {
            location.replace('/login');
        }
    } catch (err) {
        toast('无法连接到服务器，请检查后端运行状态。', 'error', 8000);
        console.error(err);
    }
}

init();
