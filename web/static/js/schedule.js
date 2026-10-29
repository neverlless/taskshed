// Shared by both pages. Task times are stored in UTC ("HH:MM", days like "Mon,Wed");
// these helpers move them into the viewer's local week, shifting the weekday when the time crosses midnight.
const DAY_KEYS = ['sun', 'mon', 'tue', 'wed', 'thu', 'fri', 'sat'];
const DAY_NAMES = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];
const WEEK_ORDER = [1, 2, 3, 4, 5, 6, 0];
const DAY_MINUTES = 1440;

function parseHM(value) {
    const m = /^(\d{1,2}):(\d{2})$/.exec((value || '').trim());
    return m ? Number(m[1]) * 60 + Number(m[2]) : null;
}

const mod = (n, m) => ((n % m) + m) % m;
const pad = n => String(n).padStart(2, '0');
// Clock time, wrapped to 00:00–23:59.
const formatHM = min => `${pad(Math.floor(mod(min, DAY_MINUTES) / 60))}:${pad(mod(min, 60))}`;
// Duration-style time that may exceed 24:00, as FullCalendar's endTime expects for windows past midnight.
const formatSpan = min => `${pad(Math.floor(min / 60))}:${pad(min % 60)}`;

function localizeTask(task) {
    const utcStart = parseHM(task.time);
    if (utcStart === null) return null;
    const duration = parseHM(task.duration);
    // shortcut: uses today's UTC offset, so windows on the far side of a DST switch show 1h off; fine for a weekly view.
    const shifted = utcStart - new Date().getTimezoneOffset();
    const dayShift = Math.floor(shifted / DAY_MINUTES);
    const start = mod(shifted, DAY_MINUTES);
    const days = [...new Set((task.days_of_week || '').split(',')
        .map(d => DAY_KEYS.indexOf(d.trim().slice(0, 3).toLowerCase()))
        .filter(d => d >= 0)
        .map(d => mod(d + dayShift, 7)))];
    return { start, end: duration ? start + duration : null, duration, days };
}

function isRunning(loc, now = new Date()) {
    if (!loc || loc.end === null) return false;
    const minute = now.getHours() * 60 + now.getMinutes();
    const today = now.getDay();
    return (loc.days.includes(today) && minute >= loc.start && minute < loc.end) ||
        (loc.end > DAY_MINUTES && loc.days.includes(mod(today - 1, 7)) && minute < loc.end - DAY_MINUTES);
}

async function fetchTasks() {
    const response = await fetch('/api/tasks');
    if (!response.ok) throw new Error(`The server responded with ${response.status} ${response.statusText}.`);
    return (await response.json()) || [];
}

// Builds DOM with textContent only, so task fields from the API can never inject markup.
function el(tag, props = {}, ...children) {
    const node = document.createElement(tag);
    for (const [key, value] of Object.entries(props)) {
        if (value === undefined || value === null || value === false) continue;
        if (key === 'className') node.className = value;
        else if (key === 'style') Object.assign(node.style, value);
        else if (key.startsWith('on')) node.addEventListener(key.slice(2), value);
        else node.setAttribute(key, value);
    }
    node.append(...children.flat().filter(c => c !== null && c !== undefined && c !== false));
    return node;
}

function showMessage(container, title, detail, action) {
    container.replaceChildren(el('div', { className: 'message', role: 'status' },
        el('p', { className: 'message-title' }, title),
        detail && el('p', {}, detail),
        action));
}

function showTimezone() {
    const zone = Intl.DateTimeFormat().resolvedOptions().timeZone;
    const offset = -new Date().getTimezoneOffset();
    const sign = offset >= 0 ? '+' : '−';
    document.getElementById('tz').textContent =
        `Times in ${zone.replace(/_/g, ' ')} (UTC${sign}${formatHM(Math.abs(offset))})`;
}
