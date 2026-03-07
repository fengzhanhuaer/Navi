/**
 * app.js — 个人导航页主逻辑（含认证流程）
 */

// ── 应用状态 ──────────────────────────────────
const state = {
    groups: [],
    sites: [],
    settings: {},
    editingGroupId: null,
};

// ── DOM 引用 ──────────────────────────────────
const $ = id => document.getElementById(id);

const DOM = {
    // 认证屏
    authScreen: $('authScreen'),
    authTitle: $('authTitle'),
    authSubtitle: $('authSubtitle'),
    authBtn: $('authBtn'),
    authUsername: $('authUsername'),
    authPassword: $('authPassword'),
    authError: $('authError'),

    // 主应用
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

    // Site modal
    siteModalBackdrop: $('siteModalBackdrop'),
    siteModalTitle: $('siteModalTitle'),
    siteForm: $('siteForm'),
    siteId: $('siteId'),
    siteTitle: $('siteTitle'),
    siteUrl: $('siteUrl'),
    siteIcon: $('siteIcon'),
    siteGroupId: $('siteGroupId'),

    // Group modal
    groupModalBackdrop: $('groupModalBackdrop'),
    groupModalTitle: $('groupModalTitle'),
    groupForm: $('groupForm'),
    groupId: $('groupId'),
    groupName: $('groupName'),
    groupIcon: $('groupIcon'),
    emojiPicker: $('emojiPicker'),

    // Settings modal
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

// ── 认证：显示登录或注册界面 ──────────────────
let authMode = 'register'; // 'register' | 'login'

function showAuth(mode) {
    authMode = mode;
    DOM.authScreen.classList.remove('hidden');
    DOM.appRoot.classList.add('hidden');
    DOM.authError.textContent = '';
    DOM.authUsername.value = '';
    DOM.authPassword.value = '';

    if (mode === 'register') {
        DOM.authTitle.textContent = '欢迎使用 Navi';
        DOM.authSubtitle.textContent = '首次使用，请先注册账号';
        DOM.authBtn.textContent = '注 册';
    } else {
        DOM.authTitle.textContent = '欢迎回来';
        DOM.authSubtitle.textContent = '请输入密码登录';
        DOM.authBtn.textContent = '登 录';
    }
    setTimeout(() => DOM.authUsername.focus(), 100);
}

function showApp() {
    DOM.authScreen.classList.add('hidden');
    DOM.appRoot.classList.remove('hidden');
}

DOM.authBtn.addEventListener('click', async () => {
    const username = DOM.authUsername.value.trim();
    const password = DOM.authPassword.value;
    DOM.authError.textContent = '';

    if (!username || !password) {
        DOM.authError.textContent = '用户名和密码不能为空';
        return;
    }

    DOM.authBtn.disabled = true;
    DOM.authBtn.textContent = '处理中...';

    try {
        let result;
        if (authMode === 'register') {
            result = await api.auth.register(username, password);
        } else {
            result = await api.auth.login(username, password);
        }
        localStorage.setItem('navi_token', result.token);
        await loadApp();
    } catch (err) {
        DOM.authError.textContent = err.message;
    } finally {
        DOM.authBtn.disabled = false;
        DOM.authBtn.textContent = authMode === 'register' ? '注 册' : '登 录';
    }
});

// 回车也可提交
DOM.authPassword.addEventListener('keydown', e => {
    if (e.key === 'Enter') DOM.authBtn.click();
});
DOM.authUsername.addEventListener('keydown', e => {
    if (e.key === 'Enter') DOM.authPassword.focus();
});

// ── 退出登录 ──────────────────────────────────
DOM.btnLogout.addEventListener('click', () => {
    if (!confirm('确定退出登录？')) return;
    localStorage.removeItem('navi_token');
    location.reload();
});

// ── 问候语 ────────────────────────────────────
function updateGreeting() {
    const h = new Date().getHours();
    const greetings = [
        [5, '凌晨好，夜猫子 🌙'],
        [9, '早上好！☀️'],
        [12, '上午好！🌤️'],
        [14, '午好！🌞'],
        [18, '下午好！🍵'],
        [22, '晚上好！🌆'],
        [24, '深夜了，注意休息 🌙'],
    ];
    const g = greetings.find(([limit]) => h < limit) || greetings[greetings.length - 1];
    DOM.greeting.textContent = g[1];
}

// ── 主题 ──────────────────────────────────────
function applyTheme(theme) {
    document.body.classList.toggle('dark', theme === 'dark');
    document.body.classList.toggle('light', theme === 'light');
    DOM.iconSun.classList.toggle('hidden', theme === 'dark');
    DOM.iconMoon.classList.toggle('hidden', theme === 'light');
}

DOM.btnTheme.addEventListener('click', async () => {
    const next = (state.settings.theme || 'dark') === 'dark' ? 'light' : 'dark';
    state.settings.theme = next;
    applyTheme(next);
    try { await api.settings.set('theme', next); } catch { }
});

// ── Favicon ───────────────────────────────────
function getFaviconUrl(siteUrl) {
    try {
        return `https://www.google.com/s2/favicons?domain=${new URL(siteUrl).origin}&sz=64`;
    } catch { return ''; }
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
    DOM.currentEngineIcon.src = ENGINES[engineKey].icon;
    DOM.currentEngineIcon.alt = ENGINES[engineKey].name;
    DOM.engineDropdown.classList.add('hidden');
    DOM.searchInput.focus();
}

// 初始化搜索引擎图标
setEngine(currentEngine);

// 切换下拉菜单
DOM.currentEngineBtn.addEventListener('click', (e) => {
    e.stopPropagation();
    DOM.engineDropdown.classList.toggle('hidden');
});

// 选择引擎
DOM.engineDropdown.querySelectorAll('.engine-option').forEach(opt => {
    opt.addEventListener('click', (e) => {
        e.stopPropagation();
        setEngine(opt.dataset.engine);
    });
});

// 点击外部关闭下拉菜单
document.addEventListener('click', e => {
    if (!DOM.searchEngineSelector.contains(e.target)) {
        DOM.engineDropdown.classList.add('hidden');
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

// ── 书签渲染 ──────────────────────────────────
function renderBookmarks() {
    if (!state.groups.length) {
        DOM.bookmarksSection.innerHTML = `
      <p style="text-align:center;color:var(--text-muted);padding:40px;">
        还没有任何分组，点击下方「新增分组」开始吧！
      </p>`;
        return;
    }

    DOM.bookmarksSection.innerHTML = state.groups.map(group => {
        const groupSites = state.sites.filter(s => s.group_id === group.id);
        const sitesHtml = groupSites.map(site => {
            const faviconUrl = site.icon || getFaviconUrl(site.url);
            const faviconEl = faviconUrl.startsWith('http')
                ? `<img src="${faviconUrl}" alt="${site.title}" onerror="this.parentElement.textContent='🌐'" />`
                : faviconUrl;
            return `
        <a class="site-card" href="${site.url}" target="_blank" data-site-id="${site.id}">
          <div class="site-favicon">${faviconEl}</div>
          <span class="site-title">${site.title}</span>
          <div class="site-actions">
            <button class="action-btn-xs btn-edit-site" data-id="${site.id}" title="编辑">✏</button>
            <button class="action-btn-xs btn-del-site"  data-id="${site.id}" title="删除">✕</button>
          </div>
        </a>`;
        }).join('');

        return `
      <div class="group-card ${group.collapsed ? 'collapsed' : ''}" data-group-id="${group.id}">
        <div class="group-header">
          <span class="group-icon">${group.icon || '📁'}</span>
          <span class="group-name">${group.name}</span>
          <span class="group-count">${groupSites.length}</span>
          <div class="group-actions">
            <button class="group-action-btn btn-add-site"  data-gid="${group.id}" title="添加网站">＋</button>
            <button class="group-action-btn btn-edit-group" data-id="${group.id}"  title="编辑分组">✏</button>
            <button class="group-action-btn danger btn-del-group" data-id="${group.id}" title="删除分组">🗑</button>
          </div>
          <span class="group-collapse-icon">▾</span>
        </div>
        <div class="sites-grid">
          ${sitesHtml}
        </div>
      </div>`;
    }).join('');

    bindBookmarkEvents();
    initSortable();
}

// ── 拖拽排序逻辑 ──────────────────────────────────
function initSortable() {
    if (typeof Sortable === 'undefined') return;

    // 1. 分组拖拽
    Sortable.create(DOM.bookmarksSection, {
        animation: 150,
        handle: '.group-header', // 仅可通过标题栏拖拽
        ghostClass: 'sortable-ghost',
        onEnd: async function () {
            const groupCards = DOM.bookmarksSection.querySelectorAll('.group-card');
            const items = Array.from(groupCards).map((el, index) => ({
                id: parseInt(el.dataset.groupId),
                order: index
            }));
            
            // 乐观更新
            items.forEach(item => {
                const g = state.groups.find(x => x.id === item.id);
                if (g) g.order_index = item.order;
            });
            state.groups.sort((a, b) => a.order_index - b.order_index);

            try {
                await api.groups.reorder(items);
            } catch (e) {
                toast('分组排序保存失败: ' + e.message, 'error');
                loadData();
            }
        }
    });

    // 2. 书签网站拖拽 (跨组)
    document.querySelectorAll('.sites-grid').forEach(grid => {
        Sortable.create(grid, {
            group: 'shared-sites', // 允许跨 grid 拖拽
            animation: 150,
            ghostClass: 'sortable-ghost',
            onEnd: async function (evt) {
                // 如果跨组拖拽了，则要更新该书签的 group_id
                const siteId = parseInt(evt.item.dataset.siteId);
                const newGroupId = parseInt(evt.to.closest('.group-card').dataset.groupId);
                const oldGroupId = parseInt(evt.from.closest('.group-card').dataset.groupId);
                
                const s = state.sites.find(x => x.id === siteId);
                if (!s) return;

                if (newGroupId !== oldGroupId) {
                    s.group_id = newGroupId;
                    // 同步到后端更新 group_id
                    try {
                        await api.sites.update(siteId, {
                            group_id: newGroupId,
                            title: s.title,
                            url: s.url,
                            icon: s.icon
                        });
                    } catch (e) {
                        toast('跨组移动失败: ' + e.message, 'error');
                        loadData();
                        return;
                    }
                }

                // 无论是同组排序还是跨组，都需要更新目标组内的所有元素顺序
                const siteCards = evt.to.querySelectorAll('.site-card');
                const items = Array.from(siteCards).map((el, index) => ({
                    id: parseInt(el.dataset.siteId),
                    order: index
                }));

                // 乐观更新
                items.forEach(item => {
                    const s = state.sites.find(x => x.id === item.id);
                    if (s) s.order_index = item.order;
                });
                state.sites.sort((a, b) => a.order_index - b.order_index);
                
                try {
                    await api.sites.reorder(items);
                } catch (e) {
                    toast('书签排序保存失败: ' + e.message, 'error');
                    loadData();
                }
            }
        });
    });
}

function bindBookmarkEvents() {
    DOM.bookmarksSection.querySelectorAll('.group-header').forEach(el => {
        el.addEventListener('click', async e => {
            if (e.target.closest('.group-actions') || e.target.closest('.btn-add-site')) return;
            const card = el.closest('.group-card');
            const gid = parseInt(card.dataset.groupId);
            const group = state.groups.find(g => g.id === gid);
            if (!group) return;
            group.collapsed = !group.collapsed;
            card.classList.toggle('collapsed', group.collapsed);
            try { await api.groups.update(gid, { name: group.name, icon: group.icon, collapsed: group.collapsed }); } catch { }
        });
    });

    DOM.bookmarksSection.querySelectorAll('.btn-add-site').forEach(el => {
        el.addEventListener('click', e => { e.preventDefault(); e.stopPropagation(); openSiteModal(null, parseInt(el.dataset.gid)); });
    });

    DOM.bookmarksSection.querySelectorAll('.btn-edit-site').forEach(el => {
        el.addEventListener('click', e => { e.preventDefault(); e.stopPropagation(); openSiteModal(parseInt(el.dataset.id)); });
    });

    DOM.bookmarksSection.querySelectorAll('.btn-del-site').forEach(el => {
        el.addEventListener('click', async e => {
            e.preventDefault(); e.stopPropagation();
            if (!confirm('确定删除这个网站吗？')) return;
            try {
                await api.sites.delete(parseInt(el.dataset.id));
                state.sites = state.sites.filter(s => s.id !== parseInt(el.dataset.id));
                renderBookmarks();
                toast('已删除', 'info');
            } catch (err) { toast('删除失败: ' + err.message, 'error'); }
        });
    });

    DOM.bookmarksSection.querySelectorAll('.btn-edit-group').forEach(el => {
        el.addEventListener('click', e => { e.stopPropagation(); openGroupModal(parseInt(el.dataset.id)); });
    });

    DOM.bookmarksSection.querySelectorAll('.btn-del-group').forEach(el => {
        el.addEventListener('click', async e => {
            e.stopPropagation();
            if (!confirm('删除分组将同时删除其中所有网站，确定吗？')) return;
            const gid = parseInt(el.dataset.id);
            try {
                await api.groups.delete(gid);
                state.groups = state.groups.filter(g => g.id !== gid);
                state.sites = state.sites.filter(s => s.group_id !== gid);
                renderBookmarks();
                toast('分组已删除', 'info');
            } catch (err) { toast('删除失败: ' + err.message, 'error'); }
        });
    });
}

// ── 分组模态框 ────────────────────────────────
const PRESET_EMOJIS = [
    '📁', '⭐️', '📚', '💼', '🎮', '🛠️', '🎵', '📺', '🛍️', '💡', '💰', '🚀', '🧠', '☁️', '🔥',
    '💻', '📱', '🎨', '📝', '🔗', '📰', '🕹️', '✉️', '📅', '🗑️', '🔍', '⚙️', '🔒', '🔑', '🌍'
];

function initEmojiPicker() {
    if (DOM.emojiPicker.children.length === 0) {
        DOM.emojiPicker.innerHTML = PRESET_EMOJIS.map(e => 
            `<button type="button" class="emoji-btn">${e}</button>`
        ).join('');
        
        DOM.emojiPicker.addEventListener('click', e => {
            if (e.target.classList.contains('emoji-btn')) {
                DOM.groupIcon.value = e.target.textContent;
            }
        });
    }
}

function openGroupModal(groupId = null) {
    state.editingGroupId = groupId;
    if (groupId) {
        const g = state.groups.find(g => g.id === groupId);
        DOM.groupModalTitle.textContent = '编辑分组';
        DOM.groupId.value = groupId;
        DOM.groupName.value = g.name;
        DOM.groupIcon.value = g.icon || '📁';
    } else {
        DOM.groupModalTitle.textContent = '新建分组';
        DOM.groupId.value = '';
        DOM.groupForm.reset();
        DOM.groupIcon.value = '📁';
    }
    initEmojiPicker();
    DOM.groupModalBackdrop.classList.remove('hidden');
    DOM.groupName.focus();
}

function closeGroupModal() { DOM.groupModalBackdrop.classList.add('hidden'); state.editingGroupId = null; }

DOM.groupForm.addEventListener('submit', async e => {
    e.preventDefault();
    const name = DOM.groupName.value.trim();
    const icon = DOM.groupIcon.value.trim() || '📁';
    try {
        if (state.editingGroupId) {
            const g = state.groups.find(g => g.id === state.editingGroupId);
            await api.groups.update(state.editingGroupId, { name, icon, collapsed: g.collapsed });
            g.name = name; g.icon = icon;
            toast('分组已更新', 'success');
        } else {
            const { id } = await api.groups.create(name, icon);
            state.groups.push({ id, name, icon, order_index: state.groups.length, collapsed: false });
            toast('分组已创建', 'success');
        }
        closeGroupModal(); renderBookmarks();
    } catch (err) { toast('保存失败: ' + err.message, 'error'); }
});

$('btnAddGroup').addEventListener('click', () => openGroupModal(null));
$('groupModalClose').addEventListener('click', closeGroupModal);
$('groupModalCancel').addEventListener('click', closeGroupModal);
DOM.groupModalBackdrop.addEventListener('click', e => { if (e.target === DOM.groupModalBackdrop) closeGroupModal(); });

// ── 网站模态框 ────────────────────────────────
function openSiteModal(siteId = null, defaultGroupId = null) {
    DOM.siteGroupId.innerHTML = state.groups.map(g =>
        `<option value="${g.id}">${g.icon} ${g.name}</option>`
    ).join('');

    if (siteId) {
        const s = state.sites.find(s => s.id === siteId);
        DOM.siteModalTitle.textContent = '编辑网站';
        DOM.siteId.value = siteId;
        DOM.siteTitle.value = s.title;
        DOM.siteUrl.value = s.url;
        DOM.siteIcon.value = s.icon || '';
        DOM.siteGroupId.value = s.group_id;
    } else {
        DOM.siteModalTitle.textContent = '添加网站';
        DOM.siteId.value = '';
        DOM.siteForm.reset();
        if (defaultGroupId) DOM.siteGroupId.value = defaultGroupId;
    }
    DOM.siteModalBackdrop.classList.remove('hidden');
    DOM.siteTitle.focus();
}

function closeSiteModal() { DOM.siteModalBackdrop.classList.add('hidden'); }

DOM.siteForm.addEventListener('submit', async e => {
    e.preventDefault();
    const siteId = DOM.siteId.value ? parseInt(DOM.siteId.value) : null;
    const data = {
        title: DOM.siteTitle.value.trim(),
        url: DOM.siteUrl.value.trim(),
        icon: DOM.siteIcon.value.trim(),
        group_id: parseInt(DOM.siteGroupId.value),
    };
    try {
        if (siteId) {
            await api.sites.update(siteId, data);
            Object.assign(state.sites.find(s => s.id === siteId), data);
            toast('已更新', 'success');
        } else {
            const { id } = await api.sites.create(data);
            state.sites.push({ ...data, id, order_index: 0 });
            toast('网站已添加', 'success');
        }
        closeSiteModal(); renderBookmarks();
    } catch (err) { toast('保存失败: ' + err.message, 'error'); }
});

$('siteModalClose').addEventListener('click', closeSiteModal);
$('siteModalCancel').addEventListener('click', closeSiteModal);
DOM.siteModalBackdrop.addEventListener('click', e => { if (e.target === DOM.siteModalBackdrop) closeSiteModal(); });

// ── 外观设置面板 ──────────────────────────────
DOM.btnSettings.addEventListener('click', () => {
    DOM.settingsModalBackdrop.classList.remove('hidden');
    DOM.settingBackground.value = state.settings.background || 'gradient';
    DOM.settingSearchEngine.value = localStorage.getItem('navi_search_engine') || 'google';
    DOM.upgradeLog.classList.add('hidden');
    DOM.upgradeLog.innerHTML = '';
});

DOM.settingBackground.addEventListener('change', async e => {
    state.settings.background = e.target.value;
    applyBackground(e.target.value);
    try { await api.settings.set('background', e.target.value); } catch { }
});

DOM.settingSearchEngine.addEventListener('change', e => {
    setEngine(e.target.value);
});

DOM.btnUpgrade.addEventListener('click', async () => {
    DOM.btnUpgrade.disabled = true;
    DOM.btnUpgrade.textContent = '升级中...';
    DOM.upgradeLog.classList.remove('hidden');
    DOM.upgradeLog.innerHTML = '<div class="log-item">正在向服务器发送升级指令...</div>';
    
    try {
        const res = await api.settings.upgrade();
        DOM.upgradeLog.innerHTML += `<div class="log-item"><pre>${res.log || '升级已完成，无日志返回'}</pre></div>`;
        toast('升级命令执行完毕，请根据日志查看是否成功。如果是重新编译更新，服务端可能将在一会后暂时断开。', 'success', 5000);
    } catch (err) {
        DOM.upgradeLog.innerHTML += `<div class="log-item ls-error">升级失败: ${err.message}</div>`;
        toast('升级失败', 'error');
    } finally {
        DOM.btnUpgrade.disabled = false;
        DOM.btnUpgrade.textContent = '检查并升级';
    }
});

$('settingsModalClose').addEventListener('click', () => DOM.settingsModalBackdrop.classList.add('hidden'));
DOM.settingsModalBackdrop.addEventListener('click', e => {
    if (e.target === DOM.settingsModalBackdrop) DOM.settingsModalBackdrop.classList.add('hidden');
});

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
    // 先检查注册状态
    const token = localStorage.getItem('navi_token');

    try {
        const { registered } = await api.auth.status();

        if (token) {
            // 有 token → 直接尝试加载（若 token 过期 api.js 会自动 reload）
            await loadApp();
        } else if (!registered) {
            // 还没有用户 → 显示注册页
            showAuth('register');
        } else {
            // 已有用户但没 token → 显示登录页
            showAuth('login');
        }
    } catch (err) {
        // 连不上服务器
        toast('无法连接到服务器', 'error', 8000);
        console.error(err);
    }
}

init();
