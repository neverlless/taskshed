// @ts-check
// Tells the server the viewer's time zone (pages are rendered in it) and labels it in the page header.

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
document.cookie = `tz=${zone}; path=/; max-age=31536000; SameSite=Lax`;

const label = document.getElementById('tz');
if (label) label.textContent = timezoneLabel(zone, new Date());
