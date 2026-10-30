// @ts-check
// Tells the server the viewer's time zone (pages are rendered in it), labels it in the page header,
// and drives the theme switch.

/**
 * Formats minutes as "HH:MM".
 * @param {number} minutes
 * @returns {string}
 */
function formatHM(minutes) {
    const pad = (/** @type {number} */ n) => String(n).padStart(2, '0');
    return `${pad(Math.floor(minutes / 60))}:${pad(minutes % 60)}`;
}

/**
 * "Times in Europe/Berlin (UTC+01:00)".
 * @param {string} zone IANA time zone name
 * @param {Date} now
 * @returns {string}
 */
function timezoneLabel(zone, now) {
    const offset = -now.getTimezoneOffset();
    const sign = offset >= 0 ? '+' : '−';
    return `Times in ${zone.replace(/_/g, ' ')} (UTC${sign}${formatHM(Math.abs(offset))})`;
}

// IANA names (letters, "/", "_", "-", "+") are valid cookie octets, so the value is stored as is.
const zone = Intl.DateTimeFormat().resolvedOptions().timeZone;
// biome-ignore lint/suspicious/noDocumentCookie: must be set synchronously before htmx's first request; cookieStore is async and not in every browser.
document.cookie = `tz=${zone}; path=/; max-age=31536000; SameSite=Lax`;

const label = document.getElementById('tz');
if (label) label.textContent = timezoneLabel(zone, new Date());

// Theme switch: the server renders the saved choice from the cookie, so this only handles clicks.
const themeSwitch = document.querySelector('.theme-switch');
if (themeSwitch instanceof HTMLElement) {
    themeSwitch.hidden = false;
    themeSwitch.addEventListener('click', (event) => {
        const button = event.target instanceof Element ? event.target.closest('button[data-theme-value]') : null;
        if (!(button instanceof HTMLButtonElement)) return;
        const theme = button.dataset.themeValue ?? '';
        if (theme) document.documentElement.dataset.theme = theme;
        else delete document.documentElement.dataset.theme;
        // biome-ignore lint/suspicious/noDocumentCookie: same reason as the tz cookie; the server reads it on the next render.
        document.cookie = theme
            ? `theme=${theme}; path=/; max-age=31536000; SameSite=Lax`
            : 'theme=; path=/; max-age=0; SameSite=Lax';
        for (const other of themeSwitch.querySelectorAll('button[data-theme-value]')) {
            other.setAttribute('aria-pressed', String(other === button));
        }
    });
}
