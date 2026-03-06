/**
 * app.js — 个人导航页主逻辑
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
    greeting: $('greeting'),
    searchInput: $('searchInput'),
    searchBtn: $('searchBtn'),
    bookmarksSection: $('bookmarksSection'),
    btnTheme: $('btnTheme'),
    iconSun: $('iconSun'),
    iconMoon: $('iconMoon'),
    btnSync: $('btnSync'),
    btnSettings: $('btnSettings'),
    toastContainer: $('toastContainer'),
    syncStatus: $('syncStatus'),

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

    // Settings modal
    settingsModalBackdrop: $('settingsModalBackdrop'),
    settingBackground: $('settingBackground'),
    d1StatusText: $('d1StatusText'),
    syncLogs: $('syncLogs'),
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

// ── 搜索（固定使用 Google）────────────────────
function doSearch() {
    const q = DOM.searchInput.value.trim();
    if (!q) return;
    if (/^https?:\/\//i.test(q) || /^[\w-]+\.\w{2,}/.test(q)) {
        window.open(/^https?:\/\//i.test(q) ? q : 'https://' + q, '_blank');
        return;
    }
    window.open(`https://www.google.com/search?q=${encodeURIComponent(q)}`, '_blank');
}

DOM.searchBtn.addEventListener('click', doSearch);
DOM.searchInput.addEventListener('keydown', e => e.key === 'Enter' && doSearch());

// 快捷键 / 聚焦搜索框
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
          <button class="add-site-card btn-add-site" data-gid="${group.id}">
            <div class="add-site-icon">＋</div>
            <span class="add-site-label">添加</span>
          </button>
        </div>
      </div>`;
    }).join('');

    bindBookmarkEvents();
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

// ── 设置面板 ──────────────────────────────────
DOM.btnSettings.addEventListener('click', async () => {
    DOM.settingsModalBackdrop.classList.remove('hidden');
    DOM.settingBackground.value = state.settings.background || 'gradient';
    try {
        const { status } = await api.sync.status();
        DOM.d1StatusText.textContent = { ok: '✅ 已连接', not_configured: '⚠️ 未配置' }[status] || '❌ 错误';
        DOM.d1StatusText.className = `status-badge ${status}`;
    } catch { DOM.d1StatusText.textContent = '❌ 请求失败'; DOM.d1StatusText.className = 'status-badge error'; }
    try {
        const logs = await api.sync.logs();
        DOM.syncLogs.innerHTML = logs.length
            ? logs.map(l => `<div class="sync-log-item">
          <span class="log-status ${l.status}">${l.status}</span>
          <span>${l.synced_at}</span><span>${l.message}</span>
        </div>`).join('')
            : '<div style="padding:10px;color:var(--text-muted);">暂无同步记录</div>';
    } catch { }
});

DOM.settingBackground.addEventListener('change', async e => {
    state.settings.background = e.target.value;
    applyBackground(e.target.value);
    try { await api.settings.set('background', e.target.value); } catch { }
});

$('settingsModalClose').addEventListener('click', () => DOM.settingsModalBackdrop.classList.add('hidden'));
DOM.settingsModalBackdrop.addEventListener('click', e => {
    if (e.target === DOM.settingsModalBackdrop) DOM.settingsModalBackdrop.classList.add('hidden');
});

$('btnManualSync').addEventListener('click', async () => {
    try { await api.sync.push(); toast('同步成功！', 'success'); }
    catch (err) { toast('同步失败: ' + err.message, 'error'); }
});

$('btnRestoreD1').addEventListener('click', async () => {
    if (!confirm('此操作将用 D1 数据覆盖本地，确定？')) return;
    try { await api.sync.restore(); toast('恢复成功，重新加载...', 'success'); setTimeout(() => location.reload(), 1500); }
    catch (err) { toast('恢复失败: ' + err.message, 'error'); }
});

// ── D1 同步按钮（顶栏）────────────────────────
DOM.btnSync.addEventListener('click', async () => {
    DOM.btnSync.classList.add('syncing');
    DOM.syncStatus.textContent = '同步中...';
    try {
        const result = await api.sync.push();
        toast(`同步成功 ✅`, 'success');
        DOM.syncStatus.textContent = `上次同步: ${new Date().toLocaleTimeString()}`;
    } catch (err) {
        toast('同步失败: ' + err.message, 'error');
        DOM.syncStatus.textContent = '同步失败';
    } finally { DOM.btnSync.classList.remove('syncing'); }
});

// ── 背景 ──────────────────────────────────────
function applyBackground(bg) { document.body.dataset.bg = bg; }

// ── 初始化 ────────────────────────────────────
async function init() {
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
        console.error('Init failed:', err);
        toast('加载失败，请检查后端服务', 'error', 6000);
        renderBookmarks();
    }
}

init();
