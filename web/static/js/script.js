document.addEventListener('DOMContentLoaded', async function() {
    const taskList = document.getElementById('task-list');
    const serviceFilter = document.getElementById('service-filter');
    const searchInput = document.getElementById('search');
    const countEl = document.getElementById('count');
    const downloadCsvButton = document.getElementById('download-csv');
    const params = new URLSearchParams(location.search);
    let sort = { column: 'start', order: 'asc' };

    showTimezone();

    let tasks;
    try {
        tasks = (await fetchTasks()).map(task => ({ ...task, loc: localizeTask(task) }));
    } catch (error) {
        showMessage(taskList, 'Tasks could not be loaded.', `${error.message} Check that the TaskShed server is running, then reload the page.`,
            el('button', { className: 'button', type: 'button', onclick: () => location.reload() }, 'Reload'));
        return;
    }

    if (tasks.length === 0) {
        showMessage(taskList, 'No tasks scheduled yet.', 'Register a task with POST /api/tasks, as described in the README, and it will appear here.');
        return;
    }

    [...new Set(tasks.map(task => task.service))].sort().forEach(service => {
        serviceFilter.append(el('option', { value: service }, service));
    });
    searchInput.value = params.get('q') || '';
    serviceFilter.value = params.get('service') || '';
    downloadCsvButton.disabled = false;

    const sortValue = {
        name: task => task.name.toLowerCase(),
        service: task => task.service.toLowerCase(),
        start: task => task.loc ? task.loc.start : Infinity,
    };

    function visibleTasks() {
        const query = searchInput.value.trim().toLowerCase();
        const service = serviceFilter.value;
        const key = sortValue[sort.column];
        const direction = sort.order === 'asc' ? 1 : -1;
        return tasks
            .filter(task => (!service || task.service === service) &&
                [task.name, task.service, task.hosts].some(v => (v || '').toLowerCase().includes(query)))
            .sort((a, b) => (key(a) < key(b) ? -1 : key(a) > key(b) ? 1 : 0) * direction);
    }

    function render() {
        const shown = visibleTasks();
        countEl.textContent = shown.length === tasks.length ? `${tasks.length} tasks` : `${shown.length} of ${tasks.length} tasks`;

        if (shown.length === 0) {
            showMessage(taskList, 'No tasks match these filters.', null,
                el('button', { className: 'button', type: 'button', onclick: clearFilters }, 'Clear filters'));
            return;
        }

        const nowMinute = new Date().getHours() * 60 + new Date().getMinutes();
        taskList.replaceChildren(el('table', { className: 'task-table' },
            el('thead', {}, el('tr', {},
                sortableHeader('name', 'Task'),
                sortableHeader('service', 'Service'),
                el('th', { scope: 'col' }, 'Days'),
                sortableHeader('start', 'Window'),
                el('th', { scope: 'col' }, 'Hosts'))),
            el('tbody', {}, shown.map(task => taskRow(task, nowMinute)))));
    }

    function sortableHeader(column, label) {
        const active = sort.column === column;
        return el('th', { scope: 'col', 'aria-sort': active ? (sort.order === 'asc' ? 'ascending' : 'descending') : 'none' },
            el('button', {
                type: 'button', className: 'sort',
                onclick: () => {
                    sort = { column, order: active && sort.order === 'asc' ? 'desc' : 'asc' };
                    render();
                },
            }, label, el('span', { className: 'sort-icon', 'aria-hidden': 'true' }, active ? (sort.order === 'asc' ? '▲' : '▼') : '')));
    }

    function taskRow(task, nowMinute) {
        const loc = task.loc;
        const running = isRunning(loc);
        return el('tr', { className: running ? 'running' : null },
            el('td', { className: 'cell-task', 'data-label': 'Task' },
                el('div', { className: 'task-name' },
                    running && el('span', { className: 'running-mark', title: 'Running now' }, el('span', { className: 'visually-hidden' }, 'Running now: ')),
                    task.name),
                task.description && el('div', { className: 'task-desc' }, task.description),
                !task.is_recurring && el('div', { className: 'task-desc' }, 'One-off task')),
            el('td', { 'data-label': 'Service' }, task.service),
            el('td', { 'data-label': task.is_recurring ? 'Days' : 'Date' }, task.is_recurring ? dayStrip(loc)
                : loc && loc.startDate ? formatDate(loc.startDate) : el('span', { className: 'task-desc' }, 'No date')),
            el('td', { className: 'cell-window', 'data-label': 'Window' }, windowCell(loc, nowMinute)),
            el('td', { className: 'cell-hosts', 'data-label': 'Hosts' }, (task.hosts || '—').split(',').map(h => h.trim()).join(', ')));
    }

    function dayStrip(loc) {
        const days = loc ? loc.days : [];
        const label = days.length ? WEEK_ORDER.filter(d => days.includes(d)).map(d => DAY_NAMES[d]).join(', ') : 'No valid days';
        return el('span', { className: 'days', role: 'img', 'aria-label': label },
            WEEK_ORDER.map(d => el('span', { className: days.includes(d) ? 'on' : null, 'aria-hidden': 'true' }, DAY_NAMES[d][0])));
    }

    function windowCell(loc, nowMinute) {
        if (!loc) return el('span', { className: 'task-desc' }, 'Invalid time');
        const text = formatWindow(loc);
        const percent = min => `${(min / DAY_MINUTES) * 100}%`;
        const segments = loc.end === null
            ? [el('span', { className: 'seg point', style: { left: percent(loc.start) } })]
            : [
                el('span', { className: 'seg', style: { left: percent(loc.start), width: percent(Math.min(loc.end, DAY_MINUTES) - loc.start) } }),
                loc.end > DAY_MINUTES && el('span', { className: 'seg', style: { left: '0', width: percent(Math.min(loc.end - DAY_MINUTES, DAY_MINUTES)) } }),
            ];
        return [
            el('div', { className: 'window-text' }, text),
            el('div', { className: 'track', 'aria-hidden': 'true' }, segments, el('span', { className: 'now', style: { left: percent(nowMinute) } })),
        ];
    }

    function clearFilters() {
        searchInput.value = '';
        serviceFilter.value = '';
        update();
    }

    function update() {
        const next = new URLSearchParams();
        if (searchInput.value.trim()) next.set('q', searchInput.value.trim());
        if (serviceFilter.value) next.set('service', serviceFilter.value);
        history.replaceState(null, '', next.size ? `?${next}` : location.pathname);
        render();
    }

    // Quote every field and neutralise leading =+-@ so spreadsheet apps don't execute task text as formulas.
    function csvField(value) {
        const text = String(value ?? '');
        return `"${(/^[=+\-@]/.test(text) ? `'${text}` : text).replace(/"/g, '""')}"`;
    }

    const isoDate = d => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;

    function downloadCSV(rows) {
        const csvContent = [
            ['Name', 'Service', 'Start (local)', 'Duration', 'End (local)', 'Days (local)', 'Date (local)', 'Recurring', 'Description', 'Hosts'],
            ...rows.map(task => [
                task.name,
                task.service,
                task.loc ? formatHM(task.loc.start) : task.time,
                task.duration || '',
                task.loc && task.loc.end !== null ? formatHM(task.loc.end) : '',
                task.loc ? WEEK_ORDER.filter(d => task.loc.days.includes(d)).map(d => DAY_NAMES[d]).join(',') : task.days_of_week,
                task.loc && task.loc.startDate ? isoDate(task.loc.startDate) : '',
                task.is_recurring ? 'yes' : 'no',
                task.description || '',
                task.hosts || '',
            ]),
        ].map(row => row.map(csvField).join(';')).join('\n');

        const url = URL.createObjectURL(new Blob([csvContent], { type: 'text/csv;charset=utf-8;' }));
        el('a', { href: url, download: 'tasks.csv' }).click();
        URL.revokeObjectURL(url);
    }

    searchInput.addEventListener('input', update);
    serviceFilter.addEventListener('change', update);
    downloadCsvButton.addEventListener('click', () => downloadCSV(visibleTasks()));
    render();
    // Keep the "now" line and running marks current while the page stays open.
    setInterval(render, 60000);
});
