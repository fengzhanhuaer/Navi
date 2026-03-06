/**
 * api.js — 与后端 API 通信的封装层
 */

const API_BASE = '/api';

async function request(method, path, body) {
    const opts = {
        method,
        headers: { 'Content-Type': 'application/json' },
    };
    if (body !== undefined) opts.body = JSON.stringify(body);
    const res = await fetch(API_BASE + path, opts);
    const data = await res.json().catch(() => ({}));
    if (!res.ok) throw new Error(data.error || `HTTP ${res.status}`);
    return data;
}

const api = {
    // 一次性加载所有初始化数据（search_engines/groups/sites/settings）
    allData: () => request('GET', '/data'),

    // 搜索引擎
    searchEngines: {
        list: () => request('GET', '/search-engines'),
        setDefault: (id) => request('PUT', `/search-engines/${id}/default`),
    },

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
};
