const state = {
  token: localStorage.getItem('admin_token') || '',
  tab: 'vacancies',
  offset: 0,
  limit: 50,
};

const app = document.getElementById('app');

function esc(s) {
  if (s == null || s === '') return '—';
  return String(s)
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;');
}

function fmtDate(v) {
  if (!v) return '—';
  return new Date(v).toLocaleString('ru-RU');
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
      <div class="card" style="margin-top:16px" id="content">Загрузка...</div>
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
    if (state.tab === 'vacancies') await renderVacancies();
    else await renderSeekers();
  } catch {
    logout();
  }
}

async function renderVacancies() {
  const data = await api(`/api/vacancies?limit=${state.limit}&offset=${state.offset}`);
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
    <table>
      <thead>
        <tr>
          <th>ID</th><th>Канал</th><th>Контакт</th><th>DM</th><th>DM время</th><th>Публикация</th><th>Текст</th>
        </tr>
      </thead>
      <tbody>${rows || '<tr><td colspan="7">Пусто</td></tr>'}</tbody>
    </table>
    ${pagerHTML(data)}
  `;
  bindPager(data);
}

async function renderSeekers() {
  const data = await api(`/api/job-seekers?limit=${state.limit}&offset=${state.offset}`);
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
    <table>
      <thead>
        <tr>
          <th>ID</th><th>Канал</th><th>Автор</th><th>Контакт</th><th>DM</th><th>DM время</th><th>Текст</th>
        </tr>
      </thead>
      <tbody>${rows || '<tr><td colspan="7">Пусто</td></tr>'}</tbody>
    </table>
    ${pagerHTML(data)}
  `;
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
    await renderApp();
  };
  if (next) next.onclick = async () => {
    state.offset = data.offset + state.limit;
    await renderApp();
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
