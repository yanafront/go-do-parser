const emptyFilters = () => ({
  q: '',
  channel: '',
  has_dm: '',
  date_from: '',
  date_to: '',
});

const state = {
  token: localStorage.getItem('admin_token') || '',
  tab: 'vacancies',
  offset: 0,
  limit: 50,
  filters: {
    vacancies: emptyFilters(),
    seekers: emptyFilters(),
  },
  channels: {
    vacancies: [],
    seekers: [],
  },
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

function currentFilters() {
  return state.filters[state.tab === 'vacancies' ? 'vacancies' : 'seekers'];
}

function channelType() {
  return state.tab === 'vacancies' ? 'vacancies' : 'seekers';
}

function buildQuery() {
  const f = currentFilters();
  const params = new URLSearchParams({
    limit: String(state.limit),
    offset: String(state.offset),
  });
  if (f.q) params.set('q', f.q);
  if (f.channel) params.set('channel', f.channel);
  if (f.has_dm) params.set('has_dm', f.has_dm);
  if (f.date_from) params.set('date_from', f.date_from);
  if (f.date_to) params.set('date_to', f.date_to);
  return params.toString();
}

async function api(path, options = {}) {
  const headers = { 'Content-Type': 'application/json', ...(options.headers || {}) };
  if (state.token) headers.Authorization = `Bearer ${state.token}`;
  const res = await fetch(path, { ...options, headers });
  const data = await res.json().catch(() => ({}));
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
      state.offset = 0;
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
  const key = state.tab === 'vacancies' ? 'vacancies' : 'seekers';
  state.filters[key] = {
    q: document.getElementById('filter-q')?.value.trim() || '',
    channel: document.getElementById('filter-channel')?.value || '',
    has_dm: document.getElementById('filter-dm')?.value || '',
    date_from: document.getElementById('filter-from')?.value || '',
    date_to: document.getElementById('filter-to')?.value || '',
  };
}

function bindFilters() {
  document.getElementById('apply-filters').onclick = async () => {
    readFiltersFromForm();
    state.offset = 0;
    await reloadTable();
  };
  document.getElementById('reset-filters').onclick = async () => {
    const key = state.tab === 'vacancies' ? 'vacancies' : 'seekers';
    state.filters[key] = emptyFilters();
    state.offset = 0;
    await reloadTable();
  };
  document.getElementById('filter-q').addEventListener('keydown', async (e) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      readFiltersFromForm();
      state.offset = 0;
      await reloadTable();
    }
  });
}

async function loadChannels() {
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
          <div class="sub">Вакансии и соискатели из Telegram</div>
        </div>
        <button class="ghost" id="logout">Выйти</button>
      </div>
      <div class="stats" id="stats"></div>
      <div class="tabs">
        <button class="tab ${state.tab === 'vacancies' ? 'active' : ''}" data-tab="vacancies">Вакансии</button>
        <button class="tab ${state.tab === 'seekers' ? 'active' : ''}" data-tab="seekers">Соискатели</button>
      </div>
      <div class="card table-card" id="content">Загрузка...</div>
    </div>
  `;
  document.getElementById('logout').onclick = logout;
  document.querySelectorAll('.tab').forEach((btn) => {
    btn.onclick = async () => {
      state.tab = btn.dataset.tab;
      state.offset = 0;
      await renderApp();
    };
  });
  try {
    const stats = await api('/api/stats');
    document.getElementById('stats').innerHTML = `
      <div class="stat"><div class="num">${stats.vacancies}</div><div class="label">Вакансии</div></div>
      <div class="stat"><div class="num">${stats.job_seekers}</div><div class="label">Соискатели</div></div>
      <div class="stat"><div class="num">${stats.dm_sent}</div><div class="label">Отправлено DM</div></div>
    `;
    await reloadTable();
  } catch {
    logout();
  }
}

async function reloadTable() {
  document.getElementById('content').innerHTML = 'Загрузка...';
  try {
    await loadChannels();
    if (state.tab === 'vacancies') await renderVacancies();
    else await renderSeekers();
  } catch {
    logout();
  }
}

async function renderVacancies() {
  const data = await api(`/api/vacancies?${buildQuery()}`);
  const rows = (data.items || []).map((v) => `
    <tr>
      <td data-label="ID">${v.id}</td>
      <td data-label="Канал">@${esc(v.source_channel)}</td>
      <td data-label="Контакт в объявлении">${esc(v.ad_username ? '@' + v.ad_username : v.ad_phone)}</td>
      <td data-label="DM кому">${esc(v.dm_contact)}</td>
      <td data-label="DM когда">${fmtDate(v.dm_sent_at)}</td>
      <td data-label="Опубликовано">${fmtDate(v.published_at)}</td>
      <td data-label="Текст" class="body-cell">${esc(v.body)}</td>
    </tr>
  `).join('');
  document.getElementById('content').innerHTML = `
    ${filtersHTML()}
    <table>
      <thead>
        <tr>
          <th>ID</th><th>Канал</th><th>Контакт</th><th>DM</th><th>DM время</th><th>Публикация</th><th>Текст</th>
        </tr>
      </thead>
      <tbody>${rows || '<tr><td colspan="7">Ничего не найдено</td></tr>'}</tbody>
    </table>
    ${pagerHTML(data)}
  `;
  bindFilters();
  bindPager(data);
}

async function renderSeekers() {
  const data = await api(`/api/job-seekers?${buildQuery()}`);
  const rows = (data.items || []).map((v) => `
    <tr>
      <td data-label="ID">${v.id}</td>
      <td data-label="Канал">@${esc(v.source_channel)}</td>
      <td data-label="Автор">@${esc(v.poster_username)}</td>
      <td data-label="Контакт">${esc(v.ad_username ? '@' + v.ad_username : v.ad_phone)}</td>
      <td data-label="DM кому">${esc(v.dm_contact)}</td>
      <td data-label="DM когда">${fmtDate(v.dm_sent_at)}</td>
      <td data-label="Текст" class="body-cell">${esc(v.body)}</td>
    </tr>
  `).join('');
  document.getElementById('content').innerHTML = `
    ${filtersHTML()}
    <table>
      <thead>
        <tr>
          <th>ID</th><th>Канал</th><th>Автор</th><th>Контакт</th><th>DM</th><th>DM время</th><th>Текст</th>
        </tr>
      </thead>
      <tbody>${rows || '<tr><td colspan="7">Ничего не найдено</td></tr>'}</tbody>
    </table>
    ${pagerHTML(data)}
  `;
  bindFilters();
  bindPager(data);
}

function pagerHTML(data) {
  const from = data.total === 0 ? 0 : data.offset + 1;
  const to = Math.min(data.offset + data.limit, data.total);
  return `
    <div class="pager">
      <span class="muted">${from}–${to} из ${data.total}</span>
      <div>
        <button class="ghost" id="prev" ${data.offset <= 0 ? 'disabled' : ''}>Назад</button>
        <button class="ghost" id="next" ${data.offset + data.limit >= data.total ? 'disabled' : ''}>Вперёд</button>
      </div>
    </div>
  `;
}

function bindPager(data) {
  const prev = document.getElementById('prev');
  const next = document.getElementById('next');
  if (prev) prev.onclick = async () => {
    state.offset = Math.max(0, data.offset - state.limit);
    await reloadTable();
  };
  if (next) next.onclick = async () => {
    state.offset = data.offset + state.limit;
    await reloadTable();
  };
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
