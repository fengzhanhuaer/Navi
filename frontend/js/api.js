/**
 * api.js — 与后端 API 通信的封装层（带 JWT）
 */

const API_BASE = '/api';

function getToken() {
    return localStorage.getItem('navi_token') || '';
}

async function request(method, path, body) {
    const opts = {
        method,
        headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${getToken()}`,
        },
    };
    if (body !== undefined) opts.body = JSON.stringify(body);
    const res = await fetch(API_BASE + path, opts);

    // 401 → 清 token，跳回首页（会触发登录界面）
    if (res.status === 401) {
        localStorage.removeItem('navi_token');
        location.reload();
        throw new Error('请重新登录');
    }

    const data = await res.json().catch(() => ({}));
    if (!res.ok) throw new Error(data.error || `HTTP ${res.status}`);
    return data;
}

// 认证（不带 token）
async function authRequest(method, path, body) {
    const opts = {
        method,
        headers: { 'Content-Type': 'application/json' },
    };
    if (body !== undefined) opts.body = JSON.stringify(body);
    const res = await fetch('/api/auth' + path, opts);
    const data = await res.json().catch(() => ({}));
    if (!res.ok) throw new Error(data.error || `HTTP ${res.status}`);
    return data;
}

const api = {
    // 认证
    auth: {
        status: () => authRequest('GET', '/status'),
        register: (username, password) => authRequest('POST', '/register', { username, password }),
        login: (username, password) => authRequest('POST', '/login', { username, password }),
    },

    // 一次性加载所有初始化数据
    allData: () => request('GET', '/data'),

    // 分组
    groups: {
        list: () => request('GET', '/groups'),
        create: (name, icon) => request('POST', '/groups', { name, icon }),
        update: (id, data) => request('PUT', `/groups/${id}`, data),
        delete: (id) => request('DELETE', `/groups/${id}`),
        reorder: (items) => request('PUT', '/groups/reorder', items),
    },

    // 网站
    sites: {
        list: (groupId) => request('GET', groupId ? `/sites?group_id=${groupId}` : '/sites'),
        create: (data) => request('POST', '/sites', data),
        update: (id, data) => request('PUT', `/sites/${id}`, data),
        delete: (id) => request('DELETE', `/sites/${id}`),
        reorder: (items) => request('PUT', '/sites/reorder', items),
    },

    // 配置
    settings: {
        get: () => request('GET', '/settings'),
        set: (key, value) => request('PUT', `/settings/${key}`, { value }),
    },

    // D1 同步
    sync: {
        push: () => request('POST', '/sync/push'),
        restore: () => request('POST', '/sync/restore'),
        status: () => request('GET', '/sync/status'),
        logs: () => request('GET', '/sync/logs'),
    },

    // D1 配置（仅输入 API Token，后端自动发现账户/数据库）
    d1: {
        configure: (apiToken) => request('POST', '/d1/configure', { api_token: apiToken }),
    },
};
