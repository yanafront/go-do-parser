const emptyFilters = () => ({
  q: '',
  channel: '',
  has_dm: '',
  date_from: '',
  date_to: '',
});

const emptyOnlinerFilters = () => ({
  q: '',
  has_contact: '',
  date_from: '',
  date_to: '',
});

const state = {
  token: localStorage.getItem('admin_token') || '',
  tab: 'vacancies',
  limit: 50,
  offsets: {
    vacancies: 0,
    seekers: 0,
    onliner: 0,
  },
  filters: {
    vacancies: emptyFilters(),
    seekers: emptyFilters(),
    onliner: emptyOnlinerFilters(),
  },
  channels: {
    vacancies: [],
    seekers: [],
  },
  loadingTable: false,
};

const app = document.getElementById('app');

function esc(s) {
  if (s == null || s === '') return '—';
  return String(s)
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;');
}

function attrEsc(s) {
  if (s == null) return '';
  return String(s)
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;');
}

function fmtDate(v) {
  if (!v) return '—';
  return new Date(v).toLocaleString('ru-RU');
}

function messageLink(row) {
  if (row.source_message_link) return row.source_message_link;
  if (row.source_channel && row.source_message_id) {
    return `https://t.me/${String(row.source_channel).replace(/^@/, '')}/${row.source_message_id}`;
  }
  return '';
}

function formatContact(username, phone) {
  if (username) {
    const u = String(username);
    if (/^[a-zA-Z][a-zA-Z0-9_]{4,31}$/.test(u)) return `@${esc(u)}`;
    return esc(u);
  }
  if (phone) return esc(phone);
  return '—';
}

function linkCell(row) {
  const url = messageLink(row);
  if (!url) return '—';
  return `<a href="${attrEsc(url)}" target="_blank" rel="noopener noreferrer">Открыть</a>`;
}

function tabKey() {
  if (state.tab === 'vacancies') return 'vacancies';
  if (state.tab === 'onliner') return 'onliner';
  return 'seekers';
}

function isOnlinerTab() {
  return state.tab === 'onliner';
}

function currentFilters() {
  return state.filters[tabKey()];
}

function currentOffset() {
  return state.offsets[tabKey()];
}

function setCurrentOffset(value) {
  state.offsets[tabKey()] = Math.max(0, Number(value) || 0);
}

function channelType() {
  return tabKey();
}

function buildQuery() {
  syncFiltersFromDOM();
  const f = currentFilters();
  const params = new URLSearchParams({
    limit: String(state.limit),
    offset: String(currentOffset()),
  });
  if (f.q) params.set('q', f.q);
  if (isOnlinerTab()) {
    if (f.has_contact) params.set('has_contact', f.has_contact);
  } else {
    if (f.channel) params.set('channel', f.channel);
    if (f.has_dm) params.set('has_dm', f.has_dm);
  }
  if (f.date_from) params.set('date_from', f.date_from);
  if (f.date_to) params.set('date_to', f.date_to);
  return params.toString();
}

function formatOnlinerContacts(row) {
  const parts = [];
  if (row.phone) parts.push(esc(row.phone));
  if (row.email) parts.push(esc(row.email));
  if (row.telegram) {
    const u = String(row.telegram);
    parts.push(/^[a-zA-Z][a-zA-Z0-9_]{4,31}$/.test(u) ? `@${esc(u)}` : esc(u));
  }
  return parts.length ? parts.join('<br>') : '—';
}

function profileCell(row) {
  if (!row.poster_profile_url) return '—';
  const label = row.poster_username || row.poster_user_id || 'Профиль';
  return `<a href="${attrEsc(row.poster_profile_url)}" target="_blank" rel="noopener noreferrer">${esc(label)}</a>`;
}

async function api(path, options = {}) {
  const headers = { 'Content-Type': 'application/json', ...(options.headers || {}) };
  if (state.token) headers.Authorization = `Bearer ${state.token}`;
  const res = await fetch(path, { ...options, headers, cache: 'no-store' });
  const data = await res.json().catch(() => ({}));
  if (res.status === 401) throw new Error('unauthorized');
  if (!res.ok) throw new Error(data.error || 'request failed');
  return data;
}

function renderLogin(error = '') {
  app.innerHTML = `
    <div class="login-wrap">
      <div class="card login-card">
        <h1>Podrabotki Admin</h1>
        <div class="sub">Вход в панель управления</div>
        <div class="error">${esc(error)}</div>
        <form id="login-form">
          <label>Пароль</label>
          <input type="password" id="password" autocomplete="current-password" required>
          <button class="primary" type="submit">Войти</button>
        </form>
      </div>
    </div>
  `;
  document.getElementById('login-form').onsubmit = async (e) => {
    e.preventDefault();
    try {
      const password = document.getElementById('password').value;
      const data = await api('/api/login', {
        method: 'POST',
        body: JSON.stringify({ password }),
      });
      state.token = data.token;
      localStorage.setItem('admin_token', state.token);
      state.offsets.vacancies = 0;
      state.offsets.seekers = 0;
      state.offsets.onliner = 0;
      await renderApp();
    } catch (err) {
      renderLogin(err.message === 'invalid password' ? 'Неверный пароль' : 'Ошибка входа');
    }
  };
}

function logout() {
  state.token = '';
  localStorage.removeItem('admin_token');
  renderLogin();
}

function filtersHTML() {
  const f = currentFilters();
  if (isOnlinerTab()) {
    return `
    <div class="filters">
      <div class="filters-row">
        <div class="field field-grow">
          <label>Поиск</label>
          <input type="search" id="filter-q" value="${attrEsc(f.q)}" placeholder="Текст, автор, телефон, email...">
        </div>
        <div class="field">
          <label>Контакт</label>
          <select id="filter-contact">
            <option value="" ${f.has_contact === '' ? 'selected' : ''}>Все</option>
            <option value="yes" ${f.has_contact === 'yes' ? 'selected' : ''}>Есть контакт</option>
            <option value="no" ${f.has_contact === 'no' ? 'selected' : ''}>Без контакта</option>
          </select>
        </div>
        <div class="field">
          <label>С даты</label>
          <input type="date" id="filter-from" value="${attrEsc(f.date_from)}">
        </div>
        <div class="field">
          <label>По дату</label>
          <input type="date" id="filter-to" value="${attrEsc(f.date_to)}">
        </div>
      </div>
      <div class="filters-actions">
        <button class="primary compact" type="button" id="apply-filters">Применить</button>
        <button class="ghost compact" type="button" id="reset-filters">Сбросить</button>
      </div>
    </div>
  `;
  }

  const channels = state.channels[channelType()] || [];
  const channelOptions = channels.map((ch) => {
    const selected = f.channel === ch ? 'selected' : '';
    return `<option value="${attrEsc(ch)}" ${selected}>@${esc(ch)}</option>`;
  }).join('');

  return `
    <div class="filters">
      <div class="filters-row">
        <div class="field field-grow">
          <label>Поиск</label>
          <input type="search" id="filter-q" value="${attrEsc(f.q)}" placeholder="Текст, @username, телефон...">
        </div>
        <div class="field">
          <label>Канал</label>
          <select id="filter-channel">
            <option value="">Все каналы</option>
            ${channelOptions}
          </select>
        </div>
        <div class="field">
          <label>DM</label>
          <select id="filter-dm">
            <option value="" ${f.has_dm === '' ? 'selected' : ''}>Все</option>
            <option value="yes" ${f.has_dm === 'yes' ? 'selected' : ''}>Отправлено</option>
            <option value="no" ${f.has_dm === 'no' ? 'selected' : ''}>Не отправлено</option>
          </select>
        </div>
        <div class="field">
          <label>С даты</label>
          <input type="date" id="filter-from" value="${attrEsc(f.date_from)}">
        </div>
        <div class="field">
          <label>По дату</label>
          <input type="date" id="filter-to" value="${attrEsc(f.date_to)}">
        </div>
      </div>
      <div class="filters-actions">
        <button class="primary compact" type="button" id="apply-filters">Применить</button>
        <button class="ghost compact" type="button" id="reset-filters">Сбросить</button>
      </div>
    </div>
  `;
}

function readFiltersFromForm() {
  const key = tabKey();
  if (key === 'onliner') {
    state.filters.onliner = {
      q: document.getElementById('filter-q')?.value.trim() || '',
      has_contact: document.getElementById('filter-contact')?.value || '',
      date_from: document.getElementById('filter-from')?.value || '',
      date_to: document.getElementById('filter-to')?.value || '',
    };
    return;
  }
  state.filters[key] = {
    q: document.getElementById('filter-q')?.value.trim() || '',
    channel: document.getElementById('filter-channel')?.value || '',
    has_dm: document.getElementById('filter-dm')?.value || '',
    date_from: document.getElementById('filter-from')?.value || '',
    date_to: document.getElementById('filter-to')?.value || '',
  };
}

function syncFiltersFromDOM() {
  if (!document.getElementById('filter-q')) return;
  readFiltersFromForm();
}

async function applyFilters() {
  readFiltersFromForm();
  setCurrentOffset(0);
  await reloadTable();
}

function bindFilters() {
  document.getElementById('apply-filters').onclick = () => applyFilters();
  document.getElementById('reset-filters').onclick = async () => {
    if (isOnlinerTab()) {
      state.filters.onliner = emptyOnlinerFilters();
    } else {
      state.filters[tabKey()] = emptyFilters();
    }
    setCurrentOffset(0);
    await reloadTable();
  };
  document.getElementById('filter-q').addEventListener('keydown', async (e) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      await applyFilters();
    }
  });
  ['filter-channel', 'filter-dm', 'filter-from', 'filter-to'].forEach((id) => {
    document.getElementById(id)?.addEventListener('change', () => applyFilters());
  });
  document.getElementById('filter-contact')?.addEventListener('change', () => applyFilters());
}

async function loadChannels() {
  if (isOnlinerTab()) return;
  const type = channelType();
  if (state.channels[type].length > 0) return;
  const data = await api(`/api/channels?type=${type}`);
  state.channels[type] = data.channels || [];
}

async function renderApp() {
  app.innerHTML = `
    <div class="container">
      <div class="topbar">
        <div>
          <h1>Podrabotki Admin</h1>
          <div class="sub">Вакансии и соискатели из Telegram и Onliner</div>
        </div>
        <button class="ghost" type="button" id="logout">Выйти</button>
      </div>
      <div class="stats" id="stats"></div>
      <div class="tabs">
        <button class="tab ${state.tab === 'vacancies' ? 'active' : ''}" type="button" data-tab="vacancies">Вакансии</button>
        <button class="tab ${state.tab === 'seekers' ? 'active' : ''}" type="button" data-tab="seekers">Соискатели TG</button>
        <button class="tab ${state.tab === 'onliner' ? 'active' : ''}" type="button" data-tab="onliner">Onliner</button>
      </div>
      <div class="card table-card" id="content">Загрузка...</div>
    </div>
  `;
  document.getElementById('logout').onclick = logout;
  document.querySelectorAll('.tab').forEach((btn) => {
    btn.onclick = async () => {
      state.tab = btn.dataset.tab;
      await renderApp();
    };
  });
  try {
    const stats = await api('/api/stats');
    document.getElementById('stats').innerHTML = `
      <div class="stat"><div class="num">${stats.vacancies}</div><div class="label">Вакансии</div></div>
      <div class="stat"><div class="num">${stats.job_seekers}</div><div class="label">Соискатели TG</div></div>
      <div class="stat"><div class="num">${stats.onliner || 0}</div><div class="label">Onliner</div></div>
      <div class="stat"><div class="num">${stats.dm_sent}</div><div class="label">Отправлено DM</div></div>
    `;
    await reloadTable();
  } catch {
    logout();
  }
}

async function reloadTable() {
  state.loadingTable = true;
  const content = document.getElementById('content');
  if (content) content.innerHTML = '<div class="loading">Загрузка...</div>';
  try {
    await loadChannels();
    if (state.tab === 'vacancies') await renderVacancies();
    else if (state.tab === 'onliner') await renderOnliner();
    else await renderSeekers();
    content?.scrollIntoView({ behavior: 'smooth', block: 'start' });
  } catch (err) {
    if (err.message === 'unauthorized') {
      logout();
      return;
    }
    if (content) {
      content.innerHTML = `<div class="error">Ошибка загрузки: ${esc(err.message)}</div>`;
    }
  } finally {
    state.loadingTable = false;
  }
}

function syncPaging(data) {
  const total = Number(data.total) || 0;
  const limit = Number(data.limit) || state.limit;
  let offset = Number(data.offset);
  if (Number.isNaN(offset)) offset = currentOffset();
  if (offset >= total && total > 0) {
    offset = Math.max(0, total - (total % limit || limit));
    if (offset === total) offset = Math.max(0, total - limit);
  }
  setCurrentOffset(offset);
  if (limit > 0) state.limit = limit;
}

async function renderVacancies() {
  const data = await api(`/api/vacancies?${buildQuery()}`);
  syncPaging(data);
  const rows = (data.items || []).map((v) => `
    <tr>
      <td data-label="ID">${v.id}</td>
      <td data-label="Канал">@${esc(v.source_channel)}</td>
      <td data-label="Ссылка" class="link-cell">${linkCell(v)}</td>
      <td data-label="Контакт в объявлении">${formatContact(v.ad_username, v.ad_phone)}</td>
      <td data-label="DM кому">${esc(v.dm_contact)}</td>
      <td data-label="DM когда">${fmtDate(v.dm_sent_at)}</td>
      <td data-label="Опубликовано">${fmtDate(v.published_at)}</td>
      <td data-label="Текст" class="body-cell">${esc(v.body)}</td>
    </tr>
  `).join('');
  document.getElementById('content').innerHTML = `
    ${filtersHTML()}
    <div class="table-wrap">
    <table>
      <thead>
        <tr>
          <th>ID</th><th>Канал</th><th>Ссылка</th><th>Контакт</th><th>DM</th><th>DM время</th><th>Публикация</th><th>Текст</th>
        </tr>
      </thead>
      <tbody>${rows || '<tr><td colspan="8">Ничего не найдено</td></tr>'}</tbody>
    </table>
    </div>
    ${pagerHTML(data)}
  `;
  bindFilters();
  bindPager();
}

async function renderOnliner() {
  const data = await api(`/api/onliner-posts?${buildQuery()}`);
  syncPaging(data);
  const rows = (data.items || []).map((v) => `
    <tr>
      <td data-label="ID">${v.id}</td>
      <td data-label="Тема">${esc(v.topic_id)}</td>
      <td data-label="Ссылка" class="link-cell">${v.topic_url ? `<a href="${attrEsc(v.topic_url)}" target="_blank" rel="noopener noreferrer">Открыть</a>` : '—'}</td>
      <td data-label="Автор">${esc(v.poster_username || v.poster_user_id)}</td>
      <td data-label="Профиль">${profileCell(v)}</td>
      <td data-label="Контакты">${formatOnlinerContacts(v)}</td>
      <td data-label="Заголовок">${esc(v.title)}</td>
      <td data-label="Текст" class="body-cell">${esc(v.body)}</td>
      <td data-label="Спарсено">${fmtDate(v.parsed_at)}</td>
    </tr>
  `).join('');
  document.getElementById('content').innerHTML = `
    ${filtersHTML()}
    <div class="table-wrap">
    <table>
      <thead>
        <tr>
          <th>ID</th><th>Тема</th><th>Ссылка</th><th>Автор</th><th>Профиль</th><th>Контакты</th><th>Заголовок</th><th>Текст</th><th>Спарсено</th>
        </tr>
      </thead>
      <tbody>${rows || '<tr><td colspan="9">Ничего не найдено</td></tr>'}</tbody>
    </table>
    </div>
    ${pagerHTML(data)}
  `;
  bindFilters();
  bindPager();
}

async function renderSeekers() {
  const data = await api(`/api/job-seekers?${buildQuery()}`);
  syncPaging(data);
  const rows = (data.items || []).map((v) => `
    <tr>
      <td data-label="ID">${v.id}</td>
      <td data-label="Канал">@${esc(v.source_channel)}</td>
      <td data-label="Ссылка" class="link-cell">${linkCell(v)}</td>
      <td data-label="Автор">${formatContact(v.poster_username, v.poster_phone)}</td>
      <td data-label="Контакт">${formatContact(v.ad_username, v.ad_phone)}</td>
      <td data-label="DM кому">${esc(v.dm_contact)}</td>
      <td data-label="DM когда">${fmtDate(v.dm_sent_at)}</td>
      <td data-label="Текст" class="body-cell">${esc(v.body)}</td>
    </tr>
  `).join('');
  document.getElementById('content').innerHTML = `
    ${filtersHTML()}
    <div class="table-wrap">
    <table>
      <thead>
        <tr>
          <th>ID</th><th>Канал</th><th>Ссылка</th><th>Автор</th><th>Контакт</th><th>DM</th><th>DM время</th><th>Текст</th>
        </tr>
      </thead>
      <tbody>${rows || '<tr><td colspan="8">Ничего не найдено</td></tr>'}</tbody>
    </table>
    </div>
    ${pagerHTML(data)}
  `;
  bindFilters();
  bindPager();
}

function pagerHTML(data) {
  const total = Number(data.total) || 0;
  const offset = currentOffset();
  const limit = state.limit;
  const from = total === 0 ? 0 : offset + 1;
  const to = Math.min(offset + limit, total);
  const page = total === 0 ? 1 : Math.floor(offset / limit) + 1;
  const pages = total === 0 ? 1 : Math.ceil(total / limit);
  const prevDisabled = offset <= 0;
  const nextDisabled = offset + limit >= total;

  return `
    <div class="pager">
      <span class="muted">${from}–${to} из ${total} · стр. ${page} из ${pages}</span>
      <div class="pager-buttons">
        <button class="ghost" type="button" id="prev" ${prevDisabled ? 'disabled' : ''}>Назад</button>
        <button class="ghost" type="button" id="next" ${nextDisabled ? 'disabled' : ''}>Вперёд</button>
      </div>
    </div>
  `;
}

function bindPager() {
  const prev = document.getElementById('prev');
  const next = document.getElementById('next');
  if (prev) {
    prev.onclick = async () => {
      if (prev.disabled) return;
      setCurrentOffset(currentOffset() - state.limit);
      await reloadTable();
    };
  }
  if (next) {
    next.onclick = async () => {
      if (next.disabled) return;
      setCurrentOffset(currentOffset() + state.limit);
      await reloadTable();
    };
  }
}

(async function init() {
  if (!state.token) {
    renderLogin();
    return;
  }
  try {
    await api('/api/stats');
    await renderApp();
  } catch {
    renderLogin();
  }
})();
