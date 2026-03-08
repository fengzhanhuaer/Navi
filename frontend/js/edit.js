/**
 * edit.js — 书签编辑页逻辑
 */

// ── 鉴权检查 ──────────────────────────────────
if (!localStorage.getItem('navi_token')) {
    location.replace('/login');
}

// ── 状态 ──────────────────────────────────────
const state = {
    groups: [],
    sites: [],
    editingGroupId: null,
    editingSiteId: null,
};

// ── 工具函数 ──────────────────────────────────
const $ = id => document.getElementById(id);

function toast(msg, type = 'info', duration = 3000) {
    const el = document.createElement('div');
    el.className = `toast ${type}`;
    el.textContent = msg;
    $('toastContainer').appendChild(el);
    setTimeout(() => {
        el.style.opacity = '0';
        el.style.transition = 'opacity .3s';
        setTimeout(() => el.remove(), 300);
    }, duration);
}

function getFaviconUrl(siteUrl) {
    return `/api/favicon?url=${encodeURIComponent(siteUrl)}`;
}

// ── 渲染分组列表 ──────────────────────────────
function render() {
    const container = $('editGroupList');

    if (!state.groups.length) {
        container.innerHTML = `<p style="text-align:center;color:var(--text-muted);padding:40px;">
            还没有分组，点击「新建分组」开始吧！</p>`;
        return;
    }

    container.innerHTML = state.groups.map(group => {
        const groupSites = state.sites.filter(s => s.group_id === group.id);

        const sitesHtml = groupSites.length
            ? groupSites.map(site => {
                // 用户手填的 emoji 直接展示；否则走本地 /api/favicon 缓存
                const isEmoji = site.icon && !site.icon.startsWith('http') && !site.icon.startsWith('/');
                const faviconSrc = isEmoji ? null : getFaviconUrl(site.url);
                const faviconEl = isEmoji
                    ? site.icon
                    : `<img src="${faviconSrc}" alt="${site.title}" data-site-id="${site.id}" onerror="this.parentElement.textContent='🌐'" />`;
                return `
          <div class="edit-site-row" data-site-id="${site.id}">
            <span class="edit-drag-handle">⠿</span>
            <div class="edit-site-favicon">${faviconEl}</div>
            <span class="edit-site-name">${site.title}</span>
            <span class="edit-site-url">${site.url}</span>
            <div class="edit-site-actions">
              <button class="edit-btn btn-refresh-icon" data-id="${site.id}" data-url="${site.url}" title="重新抓取图标">🔄</button>
              <button class="edit-btn btn-edit-site" data-id="${site.id}">✏️ 编辑</button>
              <button class="edit-btn danger btn-del-site" data-id="${site.id}">🗑 删除</button>
            </div>
          </div>`;
            }).join('')
            : `<div class="empty-group-hint">暂无网站，点击下方「添加网站」</div>`;

        return `
      <div class="edit-group-card" data-group-id="${group.id}">
        <div class="edit-group-header">
          <span class="edit-drag-handle">⠿</span>
          <span class="edit-group-icon">${group.icon || '📁'}</span>
          <span class="edit-group-name">${group.name}</span>
          <span style="font-size:.78rem;color:var(--text-muted);margin-right:8px;">${groupSites.length} 个网站</span>
          <div class="edit-group-actions">
            <button class="edit-btn btn-edit-group" data-id="${group.id}">✏️ 编辑分组</button>
            <button class="edit-btn danger btn-del-group" data-id="${group.id}">🗑 删除</button>
          </div>
        </div>
        <div class="edit-sites-list" data-group-id="${group.id}">
          ${sitesHtml}
        </div>
        <div class="edit-add-site-row">
          <button class="edit-btn btn-add-site" data-gid="${group.id}">➕ 添加网站</button>
        </div>
      </div>`;
    }).join('');

    bindEvents();
    initSortable();
}

// ── 绑定事件 ──────────────────────────────────
function bindEvents() {
    // 添加网站
    document.querySelectorAll('.btn-add-site').forEach(btn => {
        btn.addEventListener('click', () => openSiteModal(null, parseInt(btn.dataset.gid)));
    });

    // 刷新图标
    document.querySelectorAll('.btn-refresh-icon').forEach(btn => {
        btn.addEventListener('click', async () => {
            const siteUrl = btn.dataset.url;
            btn.textContent = '⏳';
            btn.disabled = true;
            try {
                const res = await fetch(`/api/favicon/refresh?url=${encodeURIComponent(siteUrl)}`, { method: 'POST' });
                const data = await res.json();
                if (data.ok) {
                    // 刷新对应的 img（加时间戳避免缓存）
                    const siteId = btn.dataset.id;
                    const img = document.querySelector(`.edit-site-favicon img[data-site-id="${siteId}"]`);
                    if (img) img.src = `/api/favicon?url=${encodeURIComponent(siteUrl)}&t=${Date.now()}`;
                    toast('图标已更新', 'success');
                } else {
                    toast('图标抓取失败: ' + (data.error || ''), 'error');
                }
            } catch (e) {
                toast('请求失败: ' + e.message, 'error');
            } finally {
                btn.textContent = '🔄';
                btn.disabled = false;
            }
        });
    });

    // 编辑网站
    document.querySelectorAll('.btn-edit-site').forEach(btn => {
        btn.addEventListener('click', () => openSiteModal(parseInt(btn.dataset.id)));
    });

    // 删除网站
    document.querySelectorAll('.btn-del-site').forEach(btn => {
        btn.addEventListener('click', async () => {
            if (!confirm('确定删除这个网站吗？')) return;
            const id = parseInt(btn.dataset.id);
            try {
                await api.sites.delete(id);
                state.sites = state.sites.filter(s => s.id !== id);
                render();
                toast('已删除', 'info');
            } catch (e) { toast('删除失败: ' + e.message, 'error'); }
        });
    });

    // 编辑分组
    document.querySelectorAll('.btn-edit-group').forEach(btn => {
        btn.addEventListener('click', () => openGroupModal(parseInt(btn.dataset.id)));
    });

    // 删除分组
    document.querySelectorAll('.btn-del-group').forEach(btn => {
        btn.addEventListener('click', async () => {
            if (!confirm('删除分组将同时删除其中所有网站，确定吗？')) return;
            const gid = parseInt(btn.dataset.id);
            try {
                await api.groups.delete(gid);
                state.groups = state.groups.filter(g => g.id !== gid);
                state.sites = state.sites.filter(s => s.group_id !== gid);
                render();
                toast('分组已删除', 'info');
            } catch (e) { toast('删除失败: ' + e.message, 'error'); }
        });
    });
}

// ── 拖拽排序 ──────────────────────────────────
function initSortable() {
    if (typeof Sortable === 'undefined') return;

    // 分组排序
    Sortable.create($('editGroupList'), {
        animation: 150,
        handle: '.edit-group-header',
        ghostClass: 'sortable-ghost',
        onEnd: async () => {
            const cards = $('editGroupList').querySelectorAll('.edit-group-card');
            const items = Array.from(cards).map((el, index) => ({
                id: parseInt(el.dataset.groupId),
                order: index,
            }));
            items.forEach(item => {
                const g = state.groups.find(x => x.id === item.id);
                if (g) g.order_index = item.order;
            });
            state.groups.sort((a, b) => a.order_index - b.order_index);
            try {
                await api.groups.reorder(items);
            } catch (e) {
                toast('排序保存失败: ' + e.message, 'error');
            }
        }
    });

    // 网站排序（跨组）
    document.querySelectorAll('.edit-sites-list').forEach(list => {
        Sortable.create(list, {
            group: 'shared-sites',
            animation: 150,
            handle: '.edit-drag-handle',
            ghostClass: 'sortable-ghost',
            onEnd: async (evt) => {
                const siteId = parseInt(evt.item.dataset.siteId);
                const newGroupId = parseInt(evt.to.dataset.groupId);
                const oldGroupId = parseInt(evt.from.dataset.groupId);
                const s = state.sites.find(x => x.id === siteId);
                if (!s) return;

                if (newGroupId !== oldGroupId) {
                    s.group_id = newGroupId;
                    try {
                        await api.sites.update(siteId, { group_id: newGroupId, title: s.title, url: s.url, icon: s.icon });
                    } catch (e) {
                        toast('跨组移动失败: ' + e.message, 'error');
                        await loadData();
                        return;
                    }
                }

                const siteRows = evt.to.querySelectorAll('.edit-site-row');
                const items = Array.from(siteRows).map((el, index) => ({
                    id: parseInt(el.dataset.siteId),
                    order: index,
                    group_id: newGroupId,
                }));
                try {
                    await api.sites.reorder(items);
                } catch (e) {
                    toast('排序保存失败: ' + e.message, 'error');
                }
            }
        });
    });
}

// ── 分组模态框 ────────────────────────────────
const PRESET_EMOJIS = [
    '📁', '⭐️', '📚', '💼', '🎮', '🛠️', '🎵', '📺', '🛍️', '💡', '💰', '🚀', '🧠', '☁️', '🔥',
    '💻', '📱', '🎨', '📝', '🔗', '📰', '🕹️', '✉️', '📅', '🗑️', '🔍', '⚙️', '🔒', '🔑', '🌍'
];

function initEmojiPicker() {
    const picker = $('emojiPicker');
    if (picker.children.length > 0) return;
    picker.innerHTML = PRESET_EMOJIS.map(e => `<button type="button">${e}</button>`).join('');
    picker.addEventListener('click', e => {
        if (e.target.tagName === 'BUTTON') $('groupIcon').value = e.target.textContent;
    });
}

function openGroupModal(groupId = null) {
    state.editingGroupId = groupId;
    $('groupModalTitle').textContent = groupId ? '编辑分组' : '新建分组';
    $('groupId').value = groupId || '';
    if (groupId) {
        const g = state.groups.find(g => g.id === groupId);
        $('groupName').value = g.name;
        $('groupIcon').value = g.icon || '📁';
    } else {
        $('groupName').value = '';
        $('groupIcon').value = '📁';
    }
    initEmojiPicker();
    $('groupModal').classList.remove('hidden');
    setTimeout(() => $('groupName').focus(), 50);
}

function closeGroupModal() { $('groupModal').classList.add('hidden'); state.editingGroupId = null; }

$('btnAddGroup').addEventListener('click', () => openGroupModal(null));
$('groupModalClose').addEventListener('click', closeGroupModal);
$('groupModalCancel').addEventListener('click', closeGroupModal);
$('groupModal').addEventListener('click', e => { if (e.target === $('groupModal')) closeGroupModal(); });

$('groupModalSave').addEventListener('click', async () => {
    const name = $('groupName').value.trim();
    const icon = $('groupIcon').value.trim() || '📁';
    if (!name) { toast('请输入分组名称', 'error'); return; }
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
        closeGroupModal();
        render();
    } catch (e) { toast('保存失败: ' + e.message, 'error'); }
});

// ── 网站模态框 ────────────────────────────────
function openSiteModal(siteId = null, defaultGroupId = null) {
    state.editingSiteId = siteId;
    $('siteModalTitle').textContent = siteId ? '编辑网站' : '添加网站';
    $('siteId').value = siteId || '';
    $('siteGroupId').innerHTML = state.groups.map(g =>
        `<option value="${g.id}">${g.icon} ${g.name}</option>`
    ).join('');

    if (siteId) {
        const s = state.sites.find(s => s.id === siteId);
        $('siteTitle').value = s.title;
        $('siteUrl').value = s.url;
        $('siteIcon').value = s.icon || '';
        $('siteGroupId').value = s.group_id;
    } else {
        $('siteTitle').value = '';
        $('siteUrl').value = '';
        $('siteIcon').value = '';
        if (defaultGroupId) $('siteGroupId').value = defaultGroupId;
    }
    $('siteModal').classList.remove('hidden');
    setTimeout(() => $('siteTitle').focus(), 50);
}

function closeSiteModal() { $('siteModal').classList.add('hidden'); state.editingSiteId = null; }

$('siteModalClose').addEventListener('click', closeSiteModal);
$('siteModalCancel').addEventListener('click', closeSiteModal);
$('siteModal').addEventListener('click', e => { if (e.target === $('siteModal')) closeSiteModal(); });

$('siteModalSave').addEventListener('click', async () => {
    const title = $('siteTitle').value.trim();
    const url = $('siteUrl').value.trim();
    const icon = $('siteIcon').value.trim();
    const group_id = parseInt($('siteGroupId').value);

    if (!title) { toast('请输入网站名称', 'error'); return; }
    if (!url) { toast('请输入网站地址', 'error'); return; }

    const data = { title, url, icon, group_id };
    try {
        if (state.editingSiteId) {
            await api.sites.update(state.editingSiteId, data);
            Object.assign(state.sites.find(s => s.id === state.editingSiteId), data);
            toast('已更新', 'success');
        } else {
            const { id } = await api.sites.create(data);
            state.sites.push({ ...data, id, order_index: 0 });
            toast('网站已添加', 'success');
        }
        closeSiteModal();
        render();
    } catch (e) { toast('保存失败: ' + e.message, 'error'); }
});

// ── 加载数据 ──────────────────────────────────
async function loadData() {
    try {
        const data = await api.allData();
        state.groups = data.groups || [];
        state.sites = data.sites || [];

        // 同步主题
        const theme = data.settings?.theme || 'dark';
        document.body.classList.toggle('dark', theme === 'dark');
        document.body.classList.toggle('light', theme === 'light');

        render();
    } catch (e) {
        $('editGroupList').innerHTML = `<p style="text-align:center;color:#ef4444;padding:40px;">加载失败: ${e.message}</p>`;
    }
}

// ── 键盘快捷键 ────────────────────────────────
document.addEventListener('keydown', e => {
    if (e.key === 'Escape') {
        closeGroupModal();
        closeSiteModal();
    }
});

// ── 启动 ──────────────────────────────────────
loadData();
